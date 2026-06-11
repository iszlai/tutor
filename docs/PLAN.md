# Tutor — Interactive AI Learning Documents

> Mobile-first web app where an LLM-generated explanation becomes a *living
> document* you can annotate, discuss, and evolve — like GitHub PR review
> comments fused with Google Docs, but the AI is a participant in every thread.

This is a planning document. It is intentionally implementation-agnostic where
the choice doesn't matter yet, and opinionated where a default will save time.

---

## 1. Product summary

1. User lands on a page with a single prompt box at the top.
2. User asks a question ("How do you calculate acceleration?").
3. The app calls an LLM and renders a concise, structured **one-pager**
   (Markdown → HTML): prose, formulas, code, tables, optionally images.
4. User **selects any span** of the document — a word, sentence, paragraph,
   formula, code block, table cell — and leaves a **comment**.
5. The AI **replies in a thread** anchored to that exact selection.
6. Each thread exposes **actions** that evolve the document:
   - **Create linked page** — spin the discussion into its own document, linked
     back to the source span.
   - **Rewrite / expand** — replace the selected content with a clearer or
     deeper version informed by the thread.
   - **Insert summary** — drop a concise summary of the thread into the doc near
     the selection.
   - **Generate visual** — diagram / illustration / chart for the selection.
   - **Generate example / exercise** — worked examples, quizzes, practice.
7. Linked pages accumulate into a **personal knowledge graph** the user can
   navigate (parent ↔ child concepts).
8. Everything persists as **Markdown + canonical JSON** so a document is fully
   portable, shareable, and reconstructable from those two files.

The north star: explanations are not static outputs but **evolving documents**.

---

## 2. Core design decisions (defaults)

These are the calls I'd make to avoid bikeshedding. Each is reversible.

| Area | Default | Why |
|------|---------|-----|
| Frontend | **React + Vite + TypeScript** | Best ecosystem for rich text selection/anchoring, mobile PWA support. |
| Styling | **Tailwind CSS** | Fast mobile-first iteration. |
| Rendering | **react-markdown + remark/rehype** plugins | Markdown is the source of truth; render to HTML with stable node mapping. |
| Math | **KaTeX** (`remark-math` + `rehype-katex`) | Formulas are first-class (the acceleration example needs this). |
| Code | **Shiki** or `rehype-highlight` | Code blocks are selectable/annotatable. |
| Diagrams | **Mermaid** (text) + raster image fallback | "Generate visual" can emit Mermaid first, image second. |
| Backend | **Node + Fastify** (or Hono) **TypeScript** | Share types with frontend; simple LLM proxying. |
| LLM | **Anthropic Claude** via official SDK, provider-abstracted | Strong structured output; abstraction keeps multi-provider future open. |
| Storage (MVP) | **SQLite** (file) + assets on disk | Zero-ops; the canonical artifacts are MD + JSON files anyway. |
| Storage (later) | Postgres + object storage (S3/R2) | When multi-user / collaboration lands. |
| Auth (MVP) | None / single-user local | Defer until needed. |
| Hosting | Static frontend + small API server; PWA installable | Mobile-first, offline-friendly later. |

> Note on this repo: the parent `inkode` repo is Go. This project is its **own
> repo** and is not bound to those choices. The defaults above optimize for the
> selection/anchoring problem, which is the hard part and is firmly in JS/DOM
> territory.

---

## 3. The hard problem: stable text anchoring

The whole experience hinges on **anchoring a comment to a selection that
survives edits, re-renders, and re-imports**. Get this right first.

### 3.1 Requirements
- Anchor a comment to an arbitrary range: word, sentence, paragraph, a formula,
  a code span, a table cell.
- Anchors must survive: re-render, document edits elsewhere, export → import.
- Degrade gracefully: if the exact text changed, mark the anchor "orphaned" but
  keep the thread (show it in a sidebar, don't lose data).

### 3.2 Approach — block IDs + character offsets + quote fallback
Persist anchors with **three redundant locators**, resolved in priority order:

1. **Block anchor** — every top-level Markdown block gets a stable `blockId`
   (ULID), stored in the JSON and as a fenced HTML comment / attribute in the
   Markdown so it round-trips. Selection records `startBlockId` / `endBlockId`.
2. **Character offset within block** — `startOffset` / `endOffset` against the
   block's *plain-text* projection (normalize whitespace deterministically).
3. **Quote fallback** — store `exactQuote` plus `prefix`/`suffix` (~32 chars
   each), à la the W3C Annotation "TextQuoteSelector". If offsets drift, re-find
   by quote+context. This is the [Hypothesis](https://web.hypothes.is/) anchoring
   strategy and it's battle-tested.

Resolution at render time: try (1)+(2); if mismatch, fall back to (3); if that
fails, mark `orphaned: true`.

### 3.3 Why not raw DOM ranges / XPath
DOM ranges and XPath break the instant the document re-renders or is edited.
Markdown-block + text-offset + quote is render-engine-independent and survives
export/import — which is a hard requirement here.

### 3.4 Library reuse
Evaluate `dom-anchor-text-quote` / `dom-anchor-text-position` (the Hypothesis
packages) for the quote/offset resolvers rather than writing from scratch. We
add the block-ID layer on top.

---

## 4. Data model (canonical JSON)

The JSON is the **source of truth for structure**; Markdown is the source of
truth for *rendered prose*. Both are emitted together and either can rebuild the
experience (Markdown alone rebuilds content; JSON adds threads/anchors/graph).

```jsonc
{
  "schemaVersion": "1.0",
  "id": "doc_01J...",                // ULID
  "title": "How to calculate acceleration",
  "rootQuestion": "How do you calculate acceleration?",
  "createdAt": "2026-06-11T10:00:00Z",
  "updatedAt": "2026-06-11T10:42:00Z",
  "provider": { "name": "anthropic", "model": "claude-..." },

  // Ordered content blocks. blockId is stable across edits.
  "blocks": [
    {
      "blockId": "blk_01J...",
      "type": "paragraph",           // paragraph|heading|formula|code|table|image|list|callout
      "markdown": "Acceleration is the rate of change of velocity...",
      "meta": {}                     // e.g. {lang:"python"} for code, {level:2} for heading
    }
  ],

  // Comment threads, each anchored to a selection.
  "threads": [
    {
      "threadId": "thr_01J...",
      "anchor": {
        "startBlockId": "blk_...",
        "endBlockId": "blk_...",
        "startOffset": 12,
        "endOffset": 13,
        "exactQuote": "v",
        "prefix": "the symbol ",
        "suffix": " stands for",
        "orphaned": false
      },
      "status": "open",              // open|resolved
      "createdAt": "...",
      "messages": [
        { "id": "msg_...", "role": "user", "text": "What does v mean?", "createdAt": "..." },
        { "id": "msg_...", "role": "assistant", "text": "v is velocity...", "createdAt": "...",
          "model": "claude-...", "tokens": { "in": 312, "out": 88 } }
      ],
      // Actions taken from this thread (audit trail + reconstruction).
      "actions": [
        { "type": "createLinkedPage", "targetDocId": "doc_...", "createdAt": "..." },
        { "type": "rewrite", "targetBlockId": "blk_...", "revisionId": "rev_...", "createdAt": "..." }
      ]
    }
  ],

  // Knowledge graph edges (this doc's outgoing links).
  "links": [
    { "type": "child", "targetDocId": "doc_...", "label": "Velocity", "sourceThreadId": "thr_...", "sourceBlockId": "blk_..." }
  ],

  // Generated assets (images, diagrams). Files stored alongside; referenced here.
  "assets": [
    { "assetId": "ast_...", "kind": "image", "path": "assets/ast_....png",
      "alt": "velocity vs time graph", "prompt": "...", "sourceThreadId": "thr_..." },
    { "assetId": "ast_...", "kind": "mermaid", "source": "graph TD; ...", "sourceThreadId": "thr_..." }
  ],

  // Version history — append-only revisions of blocks/threads.
  "revisions": [
    { "revisionId": "rev_...", "at": "...", "kind": "blockRewrite",
      "blockId": "blk_...", "before": "...", "after": "...", "byThreadId": "thr_..." }
  ]
}
```

### 4.1 Markdown round-trip
- Block IDs are embedded so they survive: emit each block followed by an HTML
  comment `<!-- blk:blk_01J... -->` (invisible in rendered output, parseable on
  import). Alternative: a YAML front-matter block-map. Pick the comment approach
  — simpler, survives copy/paste better.
- Threads/assets/graph live **only** in JSON (Markdown stays clean and
  human-readable). Sharing *just* the `.md` gives a readable doc; sharing
  `.md` + `.json` gives the full interactive experience.

### 4.2 On-disk layout (per document)
```
my-acceleration-doc/
  document.md
  document.json
  assets/
    ast_01J....png
```
A whole knowledge graph is a folder of these, plus a top-level `graph.json`
index (or derive the graph by scanning each doc's `links`).

---

## 5. LLM integration

### 5.1 Calls
| Trigger | Prompt shape | Output |
|---------|--------------|--------|
| Initial question | "Produce a concise one-page explanation in Markdown. Use headings, formulas (KaTeX `$...$`), code where helpful." | Markdown → parsed into blocks. |
| Thread reply | System: doc context + selected span + thread history. | Markdown reply (short). |
| Create linked page | "Expand `<topic>` into its own one-pager." Seeded by thread. | New document. |
| Rewrite/expand | "Rewrite this block to be simpler/deeper, incorporating: `<thread>`." | Replacement Markdown for the block. |
| Insert summary | "Summarize this thread in 1–2 sentences." | Markdown snippet → new block. |
| Generate visual | "Produce a Mermaid diagram for `<topic>`," or image-gen prompt. | Mermaid source or image. |
| Examples/exercises | "Generate N worked examples / a short quiz for `<topic>`." | Markdown block(s). |

### 5.2 Context budget
Send the **selected span + its block + nearby blocks + thread history**, not the
whole document, to keep prompts tight. For whole-doc operations, send the
Markdown.

### 5.3 Structured output
Ask the model to return Markdown only (no preamble). For actions that mutate
specific blocks, wrap the result so the app knows *what* to replace
(`targetBlockId` is decided app-side; the model just returns the new content).

### 5.4 Provider abstraction
One `LLMProvider` interface (`generate`, `stream`) with a Claude implementation
first. Streaming responses into threads/blocks for snappy mobile UX.

### 5.5 Image generation
Pluggable: start with Mermaid (free, text, versionable). Add a raster
image-gen provider behind the same `assets` abstraction. Store generated images
as files referenced from JSON — never inline base64 in the JSON (keeps it small
and diff-friendly).

---

## 6. UX / mobile-first

- **Top prompt box**, big tap target, submit on Enter (and a send button for
  touch). Streams the answer in below.
- **Selection on touch**: native long-press selection is fiddly. Provide a
  floating "💬 Comment" button that appears when a selection exists
  (`selectionchange` listener). Consider a "tap word to select, drag handles to
  extend" affordance.
- **Threads**: inline highlight markers in the doc; tapping opens the thread in
  a **bottom sheet** (mobile) or **right rail** (desktop). Don't try to render
  threads inline on a phone.
- **Action buttons** live at the bottom of each thread sheet.
- **Highlights**: rendered as `<mark>` spans injected at resolve time over the
  rendered HTML (don't store HTML; recompute from anchors each render).
- **Graph nav**: breadcrumb of parent docs + chips for child/linked pages.
  A full graph visualization is a later enhancement.
- **PWA**: installable, works as a single-purpose mobile app.

---

## 7. Architecture

```
┌─────────────────────────────────────────────┐
│  Web client (React PWA)                       │
│  - Markdown render + block-ID mapping         │
│  - Selection capture → anchor builder         │
│  - Thread UI (bottom sheet / rail)            │
│  - Action buttons → API                       │
│  - Local cache (IndexedDB) for offline read   │
└───────────────┬───────────────────────────────┘
                │ REST/JSON (SSE for streaming)
┌───────────────▼───────────────────────────────┐
│  API server (Fastify/TS)                       │
│  - LLMProvider (Claude)                        │
│  - Document service (build/edit blocks)        │
│  - Anchor resolver                             │
│  - Persistence: write MD + JSON + assets       │
│  - Asset/image generation                      │
└───────────────┬───────────────────────────────┘
                │
┌───────────────▼───────────────────────────────┐
│  Storage                                       │
│  - SQLite index (MVP) + files on disk          │
│  - or Postgres + object store (later)          │
└────────────────────────────────────────────────┘
```

### Key API endpoints (MVP)
- `POST /documents` `{ question }` → creates doc, streams Markdown.
- `GET /documents/:id` → JSON.
- `POST /documents/:id/threads` `{ anchor, message }` → creates thread, streams AI reply.
- `POST /threads/:id/messages` `{ text }` → continue thread, streams reply.
- `POST /threads/:id/actions` `{ type, params }` → run action (linked page / rewrite / summary / visual / exercise).
- `GET /documents/:id/export` → zip of `document.md` + `document.json` + `assets/`.
- `POST /documents/import` → restore from md+json (+assets).

---

## 8. Build phases

### Phase 0 — Spike the anchor (1–2 days)
Prove the hard part in isolation: render Markdown, assign block IDs, capture a
touch selection, build an anchor, persist it, re-render, re-resolve the
highlight. **Do not build anything else until this is solid.**

### Phase 1 — One-pager generation
Prompt box → Claude → Markdown → block parsing → render with KaTeX/code. Persist
MD + JSON. No comments yet.

### Phase 2 — Comments + AI threads
Selection → comment → AI reply in thread. Threads persist in JSON. Bottom-sheet
UI. This is the core loop ("what does v mean?").

### Phase 3 — Thread actions
Rewrite/expand, insert summary, generate visual (Mermaid first), examples/
exercises. Each action records to `revisions`/`actions`.

### Phase 4 — Linked pages + knowledge graph
"Create linked page" spawns a child doc with back-links. Breadcrumb + child
chips navigation. `graph.json` index.

### Phase 5 — Portability
Export/import (md+json+assets zip). Verify a doc rebuilds fully — round-trip
test is the acceptance gate.

### Phase 6 — Polish + PWA
Offline read (IndexedDB), installable PWA, streaming everywhere, image-gen
provider.

### Later (from "Future Enhancements")
Real-time / multi-user collaboration (CRDT — Yjs — once anchoring is proven),
multiple providers, branching, cross-doc search, PDF export, learning paths,
recommendations.

---

## 9. Risks & mitigations

| Risk | Mitigation |
|------|------------|
| Anchors drift after edits | 3-locator strategy + orphan state; never lose a thread. |
| Touch selection UX is poor | Custom selection affordance + floating comment button; test on real phones early. |
| Rewrites invalidate other anchors in the same block | Re-resolve all anchors after any block mutation; mark casualties orphaned, surface them. |
| JSON bloat from inline assets | Assets are files referenced by path, never embedded. |
| LLM returns non-Markdown / chatty output | Strict prompts, post-parse validation, retry. |
| Markdown ↔ JSON divergence | JSON is structural source of truth; regenerate Markdown from blocks on every save (don't hand-edit both). |
| Vendor lock-in | `LLMProvider` interface from day one. |

---

## 10. Open questions (decide before/while building)

1. **Single-user MVP or auth from the start?** (Default: single-user local.)
2. **Are blocks editable by hand**, or only via AI actions? (Default: AI-only at
   first; manual editing is a later feature that complicates anchoring.)
3. **Image generation provider** — Mermaid-only for MVP, or wire a raster
   provider immediately? (Default: Mermaid first.)
4. **Graph scope** — folder-of-docs on disk, or DB-backed from the start?
   (Default: files + SQLite index.)
5. **How aggressive is rewrite** — replace in place vs. propose-and-confirm?
   (Default: propose-and-confirm to protect anchors.)

---

## 11. Definition of done (MVP)

- Ask a question → get a rendered one-pager with a formula.
- Select "v" → comment → AI replies in an anchored thread.
- Run "rewrite", "insert summary", "generate visual", "create linked page".
- Navigate from a doc to its linked child and back.
- Export the doc, delete it, re-import md+json+assets, and have **every thread,
  reply, highlight, asset, and link restored**.
