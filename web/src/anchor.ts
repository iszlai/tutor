// Stable text anchoring. See docs/PLAN.md §3.
//
// We anchor against the *rendered* plain text of a block element. Offsets and
// the exact quote (plus surrounding context) are computed once at selection
// time and stored; at render time we resolve them back to a DOM Range, falling
// back to a quote search if offsets have drifted.

import type { Anchor } from "./types";

const CONTEXT = 32;

export function blockElement(node: Node | null): HTMLElement | null {
  let el = node instanceof HTMLElement ? node : node?.parentElement ?? null;
  while (el && !el.dataset.blockId) el = el.parentElement;
  return el;
}

// Cumulative plain-text offset of (node, offset) within root.
function offsetWithin(root: HTMLElement, node: Node, nodeOffset: number): number {
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  let total = 0;
  let n: Node | null = walker.nextNode();
  while (n) {
    if (n === node) return total + nodeOffset;
    total += n.textContent?.length ?? 0;
    n = walker.nextNode();
  }
  return total;
}

// Map a plain-text offset back to a DOM position within root.
function positionAt(root: HTMLElement, target: number): { node: Node; offset: number } | null {
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  let total = 0;
  let n: Node | null = walker.nextNode();
  while (n) {
    const len = n.textContent?.length ?? 0;
    if (target <= total + len) return { node: n, offset: target - total };
    total += len;
    n = walker.nextNode();
  }
  return null;
}

export function buildAnchor(sel: Selection): Anchor | null {
  if (sel.rangeCount === 0 || sel.isCollapsed) return null;
  const range = sel.getRangeAt(0);
  const startEl = blockElement(range.startContainer);
  const endEl = blockElement(range.endContainer);
  if (!startEl || startEl !== endEl) return null; // MVP: single-block selections

  const text = startEl.textContent ?? "";
  const start = offsetWithin(startEl, range.startContainer, range.startOffset);
  const end = offsetWithin(startEl, range.endContainer, range.endOffset);
  if (end <= start) return null;

  const id = startEl.dataset.blockId!;
  return {
    startBlockId: id,
    endBlockId: id,
    startOffset: start,
    endOffset: end,
    exactQuote: text.slice(start, end),
    prefix: text.slice(Math.max(0, start - CONTEXT), start),
    suffix: text.slice(end, end + CONTEXT),
  };
}

// Resolve an anchor to a DOM Range inside its block element, or null if orphaned.
export function resolveRange(root: HTMLElement, anchor: Anchor): Range | null {
  const text = root.textContent ?? "";

  let start = anchor.startOffset;
  let end = anchor.endOffset;

  // Trust offsets only if the quote still matches there; otherwise re-find it.
  if (text.slice(start, end) !== anchor.exactQuote) {
    const idx = findByQuote(text, anchor);
    if (idx < 0) return null;
    start = idx;
    end = idx + anchor.exactQuote.length;
  }

  const a = positionAt(root, start);
  const b = positionAt(root, end);
  if (!a || !b) return null;

  const range = document.createRange();
  try {
    range.setStart(a.node, a.offset);
    range.setEnd(b.node, b.offset);
  } catch {
    return null;
  }
  return range;
}

// Prefer the occurrence nearest the original offset, biased by context.
function findByQuote(text: string, anchor: Anchor): number {
  if (!anchor.exactQuote) return -1;
  const ctx = anchor.prefix + anchor.exactQuote;
  const ctxIdx = text.indexOf(ctx);
  if (ctxIdx >= 0) return ctxIdx + anchor.prefix.length;
  return text.indexOf(anchor.exactQuote);
}

// Paint highlights for all resolvable threads using the CSS Custom Highlight API.
// Returns the set of threadIds that resolved (the rest are orphaned).
export function paintHighlights(
  container: HTMLElement,
  anchors: { threadId: string; anchor: Anchor }[]
): Set<string> {
  const resolved = new Set<string>();
  const cssHighlights = (CSS as unknown as { highlights?: Map<string, unknown> }).highlights;
  if (!cssHighlights || typeof (window as unknown as { Highlight?: unknown }).Highlight === "undefined") {
    return resolved; // Browser without the API — markers still work, just no inline tint.
  }

  const HighlightCtor = (window as unknown as { Highlight: new (...r: Range[]) => unknown })
    .Highlight;
  const ranges: Range[] = [];
  for (const { threadId, anchor } of anchors) {
    const block = container.querySelector<HTMLElement>(`[data-block-id="${anchor.startBlockId}"]`);
    if (!block) continue;
    const range = resolveRange(block, anchor);
    if (range) {
      ranges.push(range);
      resolved.add(threadId);
    }
  }
  cssHighlights.set("tutor-comment", new HighlightCtor(...ranges));
  return resolved;
}

const ANNOTATION_KINDS = ["gap", "jargon", "shaky"] as const;

// Paint Feynman annotations as inline highlights, color-coded by kind, by
// resolving each verbatim quote within its block (reusing the anchor machinery).
// Always sets every kind's highlight so stale ranges from a prior doc clear.
export function paintAnnotations(
  container: HTMLElement,
  items: { blockId: string; quote: string; kind: string }[]
): void {
  const cssHighlights = (CSS as unknown as { highlights?: Map<string, unknown> }).highlights;
  if (!cssHighlights || typeof (window as unknown as { Highlight?: unknown }).Highlight === "undefined") {
    return;
  }
  const HighlightCtor = (window as unknown as { Highlight: new (...r: Range[]) => unknown })
    .Highlight;

  const byKind: Record<string, Range[]> = { gap: [], jargon: [], shaky: [] };
  for (const it of items) {
    const ranges = byKind[it.kind];
    if (!ranges) continue;
    const block = container.querySelector<HTMLElement>(`[data-block-id="${it.blockId}"]`);
    if (!block) continue;
    const range = resolveRange(block, {
      startBlockId: it.blockId,
      endBlockId: it.blockId,
      startOffset: 0,
      endOffset: 0,
      exactQuote: it.quote,
      prefix: "",
      suffix: "",
    });
    if (range) ranges.push(range);
  }
  for (const kind of ANNOTATION_KINDS) {
    cssHighlights.set(`tutor-fey-${kind}`, new HighlightCtor(...byKind[kind]));
  }
}
