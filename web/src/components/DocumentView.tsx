import { useEffect, useRef, useState } from "react";
import type { Annotation, DocLink, Thread, TutorDoc } from "../types";
import { paintHighlights, markAnnotations } from "../anchor";
import { Markdown } from "./Markdown";

const KIND_LABEL: Record<string, string> = {
  gap: "vague / missing",
  jargon: "undefined jargon",
  shaky: "looks shaky",
};

type NotePopover = { text: string; kind: string; x: number; y: number };

export function DocumentView({
  doc,
  onOpenThread,
  onNavigate,
}: {
  doc: TutorDoc;
  onOpenThread: (threadId: string) => void;
  onNavigate: (docId: string) => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const [note, setNote] = useState<NotePopover | null>(null);

  // Repaint anchored-comment highlights and Feynman marks whenever the doc changes.
  useEffect(() => {
    if (!ref.current) return;
    setNote(null);
    paintHighlights(
      ref.current,
      doc.threads.map((t) => ({ threadId: t.threadId, anchor: t.anchor }))
    );
    // Feynman: wrap the flagged phrases inside the learner's explanations so
    // they can be tapped for the note.
    markAnnotations(
      ref.current,
      doc.blocks.flatMap((b) =>
        Array.isArray(b.meta?.annotations)
          ? (b.meta!.annotations as Annotation[]).map((a) => ({
              blockId: b.blockId,
              quote: a.quote,
              kind: a.kind,
              note: a.note,
            }))
          : []
      )
    );
  }, [doc]);

  // Tap a Feynman mark to reveal its note; tap elsewhere in the doc to dismiss.
  function onDocClick(e: React.MouseEvent) {
    const mark = (e.target as HTMLElement).closest<HTMLElement>(".fey-mark");
    if (!mark) {
      setNote(null);
      return;
    }
    const r = mark.getBoundingClientRect();
    setNote({
      text: mark.dataset.note ?? "",
      kind: mark.dataset.kind ?? "gap",
      x: r.left + r.width / 2,
      y: r.bottom + 6,
    });
  }

  const threadsByBlock = new Map<string, Thread[]>();
  for (const t of doc.threads) {
    const list = threadsByBlock.get(t.anchor.startBlockId) ?? [];
    list.push(t);
    threadsByBlock.set(t.anchor.startBlockId, list);
  }

  // Child links with a sourceBlockId appear inline at that block.
  // Others (and related links) appear in the breadcrumb.
  const inlineLinks = new Map<string, DocLink[]>();
  for (const l of doc.links) {
    if (l.type === "child" && l.sourceBlockId) {
      const list = inlineLinks.get(l.sourceBlockId) ?? [];
      list.push(l);
      inlineLinks.set(l.sourceBlockId, list);
    }
  }
  const breadcrumbChildren = doc.links.filter((l) => l.type === "child" && !l.sourceBlockId);
  const related = doc.links.filter((l) => l.type === "related");

  return (
    <div>
      <h1 className="doc-title">{doc.title}</h1>

      {(breadcrumbChildren.length > 0 || related.length > 0) && (
        <div className="breadcrumb">
          {related.map((l) => (
            <LinkChip key={l.targetDocId} link={l} prefix="↑" onNavigate={onNavigate} />
          ))}
          {breadcrumbChildren.map((l) => (
            <LinkChip key={l.targetDocId} link={l} prefix="↳" onNavigate={onNavigate} />
          ))}
        </div>
      )}

      <div ref={ref} onClick={onDocClick}>
        {doc.blocks.map((b) => {
          const threads = threadsByBlock.get(b.blockId) ?? [];
          const blockLinks = inlineLinks.get(b.blockId) ?? [];
          return (
            <div
              key={b.blockId}
              className={`block${b.meta?.feynman === "explanation" ? " block--feynman" : ""}`}
              data-block-id={b.blockId}
            >
              <Markdown>{b.markdown}</Markdown>
              {threads.map((t) => (
                <span
                  key={t.threadId}
                  className="thread-marker"
                  onClick={() => onOpenThread(t.threadId)}
                  title="Open discussion"
                >
                  💬 {t.messages.filter((m) => m.role === "user").length}
                </span>
              ))}
              {blockLinks.map((l) => (
                <span
                  key={l.targetDocId}
                  className="inline-link-chip"
                  onClick={() => onNavigate(l.targetDocId)}
                  title={l.label}
                >
                  ↳ {l.label}
                </span>
              ))}
            </div>
          );
        })}
      </div>

      {note && (
        <>
          <div className="fey-popover-scrim" onClick={() => setNote(null)} />
          <div className="fey-popover" style={{ left: note.x, top: note.y }}>
            <span className={`fey-popover-kind fey-popover-kind--${note.kind}`}>
              {KIND_LABEL[note.kind] ?? note.kind}
            </span>
            {note.text}
          </div>
        </>
      )}
    </div>
  );
}

function LinkChip({
  link,
  prefix,
  onNavigate,
}: {
  link: DocLink;
  prefix: string;
  onNavigate: (id: string) => void;
}) {
  return (
    <span className="chip" onClick={() => onNavigate(link.targetDocId)}>
      {prefix} {link.label}
    </span>
  );
}
