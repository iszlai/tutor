# Feynman Mode — implementation plan & progress

Branch: `claude/hungarian-translation-support-7jdoaw`

## Goal
Add a "Teach (Feynman)" mode: instead of the AI explaining, the **learner**
explains a concept in plain words and the AI plays a **supportive coach** that
surfaces gaps. Iterate in passes until the explanation is solid.

Decisions (from the user):
- **Entry:** freeform topic — toggle the prompt box to Teach mode, name a
  topic, write your explanation, get a gap report. (Not "Feynman an existing
  doc" — that's a later add.)
- **Tone:** supportive coach (encouraging, hints not blunt verdicts).

## Design
A Feynman session is a `TutorDoc` with `Mode: "feynman"`. Each pass appends:
1. a **callout block** holding the learner's explanation (meta
   `{feynman:"explanation", round:N}`), then
2. the AI **gap report** parsed into normal blocks.

So the loop is just blocks → search, rendering, persistence all reused.

Gap report sections (level-3 headings): **What landed / Gaps to fill / Jargon to
unpack / One question to sit with / Where you are**. Responds in the learner's
language (Hungarian-aware). No numeric score.

### Endpoints
- `POST /api/feynman` `{topic, explanation}` → streams report (SSE), creates the
  doc, ends with `data: [DONE] <docId>`.
- `POST /api/documents/{id}/feynman` `{explanation}` → streams a new pass,
  appends blocks, ends with `data: [DONE]`. Frontend re-fetches the doc.

## Progress checklist

### Backend — DONE (builds + tests pass)
- [x] `types.go`: `TutorDoc.Mode` field.
- [x] `prompts.go`: `feynmanSystem` (supportive coach) + `feynmanPrompt(topic, explanation, prior)`.
- [x] `helpers.go`: SSE helpers `sseHeaders` / `sseToken` / `sseError` (added `fmt` import).
- [x] `feynman.go`: `createFeynman`, `feynmanRound`, `feynmanExplanationBlock`,
      `feynmanRoundCount`, `sessionText`.
- [x] `server.go`: routes for the two endpoints.

### Frontend — DONE (tsc + vite build clean)
- [x] `types.ts`: `TutorDoc.mode?`.
- [x] `api.ts`: `streamSSE` helper + `createFeynmanStream` + `feynmanRoundStream`.
- [x] `components/TeachBox.tsx`: composer (topic optional + explanation textarea).
- [x] `App.tsx`: `mode` toggle, `teach()` + `refine()`, streaming reuse, refine
      box for feynman docs, import toggle hidden in Teach mode.
- [x] `components/DocumentView.tsx`: `block--feynman` class for explanation blocks.
- [x] `styles.css`: `.mode-toggle`, `.teach*`, `.block--feynman*`, refine area.

### Verified
- [x] Mock-LLM smoke test: create → `mode:feynman`; round appends a 2nd
      explanation block. Both endpoints stream + return as expected.

### Finish
- [x] Commit + push to the branch.

## Anchored gap highlighting — DONE (builds, tests pass)
After the gap report, a best-effort structured pass (`annotateExplanation`) asks
the model for verbatim phrases to flag, returned as JSON and parsed leniently
(`parseAnnotations` drops hallucinated quotes / defaults bad kinds). Stored on
the explanation block's `meta.annotations` as `[{quote, kind, note}]`.

Frontend `markAnnotations` (anchor.ts) resolves each quote to a DOM range via
the existing `resolveRange` and wraps it in a `<mark class="fey-mark fey-mark--KIND">`
(note on `data-note`). Real elements → tappable. DocumentView calls it on repaint
(idempotent: skips blocks already marked), and a delegated click handler opens a
small note popover. App shows a color legend above Feynman docs.

Earlier iteration used the CSS Custom Highlight API (`::highlight(tutor-fey-*)`),
but that can't host tap interaction, so it was replaced by wrapped marks. Thread
comments still use the Highlight API via `paintHighlights`.
- Backend: `Annotation` type, `annotateSystem` prompt, `annotateExplanation`,
  `parseAnnotations` (+ test), wired into both handlers via
  `feynmanExplanationBlock(expl, round, anns)`.
- Frontend: `Annotation` type, `paintAnnotations`, DocumentView wiring, legend,
  `::highlight(tutor-fey-*)` + `.fey-chip*` CSS.

## Tap-to-see-note — DONE
Annotations are wrapped marks (see above); tapping one opens a popover with the
note + kind label. Tapping elsewhere dismisses. `web/vite build` clean.

## Potential follow-ups (not built — backlog)

### 1. "Feynman an existing doc"
A second entry point: from a normal Tutor doc you've already explored, start a
Feynman session *about that doc* — hide the AI's explanation, prompt you to
re-explain it yourself, then grade your attempt against the original as the
source of truth.
- Likely a thread/doc action ("Feynman this") that seeds a feynman doc with a
  `Links` "related" back to the source, and passes the source text as grading
  context to `feynmanPrompt`/`annotateExplanation`.
- Open question: reveal the source after the first pass, or keep it hidden until
  you ask?

### 2. Clarity progression across passes
Show how the explanation improves over passes — e.g. a per-pass read (gaps
remaining / jargon count, or a coach "clarity" line) and a small trend so the
learner sees momentum.
- Data is already there: each pass = one explanation block + its annotations;
  count annotations per pass for a cheap first version.
- Could surface as a header strip on the feynman doc, or a sparkline near the
  legend.

## Resume notes
- SSE frame format the frontend expects: `data: <json-token>\n\n`, terminal
  `data: [DONE] <payload>\n\n`, errors `data: [ERROR] <msg>\n\n`.
- Existing streaming references: `createDocStream` (App `ask`) and `replyStream`
  (App `sheetSend`) — mirror their UX with `streamingDoc` / `streamingReply`.
- Run from `web/`: `npx tsc --noEmit && npx vite build`. Backend: `cd server &&
  go build ./... && go test ./...`.
</content>
