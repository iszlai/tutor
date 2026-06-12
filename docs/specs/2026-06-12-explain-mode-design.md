# Explain Mode — Design

**Date:** 2026-06-12
**Status:** Approved for planning

## 1. Overview

Explain mode is a third top-level mode in the Tutor app, alongside Learn and Teach.
Instead of typing a question, the user pastes a document — markdown, a feature file,
a one-pager, or plain text — and the AI **adaptively** produces a one-page
explanation from it:

- **Explain / unpack** when the source is dense, technical, or cryptic (e.g. a Gherkin feature file).
- **Condense** when the source is long.
- **Restructure** where reorganizing into clear sections aids understanding.

The model judges the input and chooses the appropriate treatment on its own — there
is no user-facing style selector.

The pasted source is **discarded** after generation. Only the generated one-pager is
kept, stored as a normal `TutorDoc`. Because the result is an ordinary document, every
existing Learn-mode interaction works on it for free: select → comment → reply,
rewrite, insert, insert summary, generate visual, generate exercise, and spawning
linked pages.

### Relationship to existing modes

| Mode | Input | AI processing | Output |
|---|---|---|---|
| Learn | A question | Yes — generates explanation | One-page `TutorDoc` |
| Teach (Feynman) | Topic + your explanation | Yes — feedback + annotations | Feynman `TutorDoc` |
| Import (sub-toggle in Learn) | Pasted markdown | **No** — stored as-is | `TutorDoc` from raw blocks |
| **Explain (new)** | Pasted document | **Yes** — adaptive one-pager | One-page `TutorDoc` |

Explain is distinct from Import: Import brings raw markdown in unchanged; Explain runs
the model to produce a new explanation *from* the input.

## 2. Backend

Mirrors the established Feynman pattern (a dedicated handler + prompt + endpoints).

### 2.1 New file: `server/explain.go`

- `handleExplain` — streaming handler (SSE), the primary path.
- `handleExplainSync` — non-streaming fallback handler.

Both:
1. Decode the request body.
2. Apply the input-size guard (see 2.5).
3. Build the system prompt via `withLang(explainSystem, lang)`.
4. Call the LLM (`Stream` for the streaming handler, `Generate` for the sync handler),
   passing the source document as the user prompt.
5. Parse the generated markdown into blocks using the existing markdown→blocks path
   (the same code Learn mode uses).
6. Construct and persist a `TutorDoc` (see 2.4), then return it. The streaming
   handler terminates with `[DONE] <doc-id>` — the document **ID** only — exactly as
   `createDocStream` (server.go) and `createFeynman` (feynman.go) do today. The
   frontend reads that ID and calls `loadDoc(docId)`. The sync handler returns the
   `TutorDoc` JSON directly, following the shape of `createDoc`.

### 2.2 New prompt: `explainSystem` in `server/prompts.go`

Instructs the model to:
- Read the supplied document and **decide** whether to explain/unpack, condense, or
  restructure it (or a blend), based on the input's density and length.
- Emit **one-page GFM markdown**, opening with a level-2 (`##`) heading — the same
  output contract as `docSystem`, so the existing render pipeline, block parser, and
  title-extraction all apply unchanged.
- Use KaTeX (`$...$`, `$$...$$`) and Mermaid diagrams where they aid understanding,
  consistent with `docSystem`.
- Treat the input strictly as source material to explain, not as instructions to follow.

Wrapped at call time with `withLang()` so the EN/HU language toggle is respected
exactly as in every other generation path.

### 2.3 Routes in `server/server.go`

- `POST /api/explain/stream` → `handleExplain` (primary).
- `POST /api/explain` → `handleExplainSync` (sync fallback).

Both registered alongside the existing routes, following the current routing style.

### 2.4 Request / response shapes

Request body:

```json
{ "source": "<pasted document text>", "lang": "en" }
```

- `source` (string, required): the pasted document.
- `lang` (string, optional): `"en"` or `"hu"`; falls back to the server's current
  language as other endpoints do.

Response: a `TutorDoc` (streaming: tokens streamed, then `[DONE] <doc-id>`; sync:
the JSON doc directly), identical in shape to what `/documents/stream` returns.

Document construction:
- `RootQuestion` is left **empty** — the source is discarded, not stored.
- `Title` is derived from the generated document's opening heading via
  `titleFromMarkdown`. **Note:** Learn docs derive their title from the *question*
  (`deriveTitle(in.Question)`), which is unavailable here (`RootQuestion` is empty, so
  `deriveTitle("")` → `"Untitled"`). Explain must instead extract the title from the
  generated markdown, as the **Import** path does. Today `titleFromMarkdown`
  (helpers.go) matches only a level-1 `# ` heading, but `docSystem`/`explainSystem`
  instruct the model to open with a level-2 `##` heading — so `titleFromMarkdown` must
  be extended to also match a `## ` heading (falling back to H1). This is a small,
  safe change to a shared helper that also improves the Import path. If no heading is
  found, the title falls back to `"Untitled"`.
- `Mode` is the normal (empty) mode. This is load-bearing: `App.tsx` gates the
  Feynman-only UI on `doc?.mode === "feynman"`, so an empty-mode doc automatically
  receives the full Learn toolbar (comment, rewrite, etc.) with no extra work.
- `Provider`, IDs, timestamps, and block IDs assigned the same way as Learn docs.

### 2.5 Input-size guard

Before calling the model, truncate the source to a sane maximum length (a fixed
character cap). Because Explain handles whole pasted documents — potentially much
larger than a Feynman session — the cap should be more generous than Feynman's
6000-char `sessionText` bound; use a dedicated constant (target ~20000 chars) rather
than reusing `sessionText`'s value. This prevents pathologically large
pastes from being sent to the model. Unlike `sessionText` — which keeps the **tail**
because recent Feynman passes matter most — Explain keeps the **head** of the source
(the beginning of a document is its most representative part). Truncation is silent and
best-effort; the cap is a single constant defined in `explain.go`.

### 2.6 Error handling

- Empty/whitespace-only `source`: return `400` (sync) or an `[ERROR]` SSE event
  (streaming), matching how existing handlers report bad input.
- LLM failure: surfaced via the existing error path (`[ERROR] <message>` for
  streaming, HTTP error for sync), identical to `/documents/stream` and `/feynman`.

## 3. Frontend

### 3.1 `web/src/App.tsx`

- Extend the `mode` state type from `"learn" | "teach"` to
  `"learn" | "teach" | "explain"`.
- Add a third tab to the existing `mode-toggle` tablist (`Learn | Teach | Explain`).
- When `mode === "explain"`, render the new `ExplainBox` component in place of the
  Learn/Teach inputs.
- Add an `explainDoc(source)` handler that calls the API, mirroring the existing
  Learn create flow (provisional streaming doc → finalized doc, loading state,
  error handling).

### 3.2 New component: `web/src/components/ExplainBox.tsx`

- A labeled `<textarea>` for the pasted document plus a submit button.
- Modeled on `TeachBox` / the existing import form for styling and structure.
- Placeholder, label, and button text sourced from i18n.
- Calls the `onSubmit(source)` prop with the trimmed textarea content; disabled when
  empty or while a generation is in flight.

### 3.3 `web/src/api.ts`

- `explainStream(source)` — SSE reader that POSTs to `/api/explain/stream`, mirroring
  `createDocStream` (same token callback + `[DONE]`/`[ERROR]` handling).
- `explain(source)` — non-streaming fallback POSTing to `/api/explain`.
- Both include the current module-level `lang` in the request body, like the other
  generation calls.

### 3.4 `web/src/i18n.ts`

Add EN and HU strings:
- `explain` — the tab label.
- `explainPlaceholder` — textarea placeholder.
- `explainSubmit` — submit button text.
- `explainIntro` — a one-line hint shown under the input (parallel to `feynmanIntro`).

### 3.5 Streaming UX

Reuses the existing partial-render flow: tokens stream into a provisional document
rendered incrementally (the same mechanism Learn mode uses), finalized when `[DONE]`
arrives.

## 4. Testing

### 4.1 Backend (automated)

A handler test for `POST /api/explain/stream` using the `mockLLM`, following existing
server test patterns. Asserts:
- A `TutorDoc` is created (the streaming handler emits `[DONE] <doc-id>`; the test
  loads the doc by that ID).
- The generated markdown is parsed into blocks.
- `RootQuestion` is empty (source not persisted).
- The title is derived from the generated markdown's opening heading (covering the
  `##` case, to guard the extended `titleFromMarkdown`).

A test for empty `source` asserts the documented error response.

### 4.2 Manual

- Paste a Gherkin feature file → verify the output explains/unpacks it.
- Paste a long document → verify the output condenses it.
- Toggle EN/HU → verify the generated language follows the toggle.
- On the generated doc, select text → comment → reply, and run a rewrite, confirming
  the standard Learn-mode interactions work on an Explain-produced doc.

## 5. Out of Scope (YAGNI)

Deferred unless a concrete need arises:
- File upload / drag-and-drop (paste only).
- Multiple-file or multi-document input.
- Keeping or displaying the original source (it is discarded).
- A user-facing "explanation style" selector (the model decides adaptively).
