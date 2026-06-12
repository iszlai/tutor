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

Frontend `paintAnnotations` (anchor.ts) resolves each quote to a DOM range via
the existing `resolveRange` and paints it with the CSS Custom Highlight API,
color-coded by kind (`tutor-fey-gap|jargon|shaky`). DocumentView calls it on
repaint; App shows a color legend above Feynman docs.
- Backend: `Annotation` type, `annotateSystem` prompt, `annotateExplanation`,
  `parseAnnotations` (+ test), wired into both handlers via
  `feynmanExplanationBlock(expl, round, anns)`.
- Frontend: `Annotation` type, `paintAnnotations`, DocumentView wiring, legend,
  `::highlight(tutor-fey-*)` + `.fey-chip*` CSS.

## Future adds (not built)
- "Feynman an existing doc" entry (hide explanation, re-explain, grade vs source).
- Hover/tap a highlighted phrase to show its note inline (the note is already
  stored; CSS Highlights can't host tooltips, so this needs span wrapping).
- A "clarity progression" across passes.

## Resume notes
- SSE frame format the frontend expects: `data: <json-token>\n\n`, terminal
  `data: [DONE] <payload>\n\n`, errors `data: [ERROR] <msg>\n\n`.
- Existing streaming references: `createDocStream` (App `ask`) and `replyStream`
  (App `sheetSend`) — mirror their UX with `streamingDoc` / `streamingReply`.
- Run from `web/`: `npx tsc --noEmit && npx vite build`. Backend: `cd server &&
  go build ./... && go test ./...`.
</content>
