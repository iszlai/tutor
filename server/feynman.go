package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Annotation marks a verbatim phrase in the learner's explanation that needs
// attention, so the frontend can highlight it inline (reusing the anchor
// quote-resolution). Stored on the explanation block's meta.
type Annotation struct {
	Quote string `json:"quote"`
	Kind  string `json:"kind"` // gap|jargon|shaky
	Note  string `json:"note"`
}

// Feynman mode: the learner teaches a concept back in their own words and Tutor
// returns a supportive gap report. A session is a TutorDoc with Mode "feynman";
// each pass appends the learner's explanation (a callout) followed by the gap
// report, so the whole loop is just blocks — search, rendering, and persistence
// come for free.

func (a *API) createFeynman(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Topic       string `json:"topic"`
		Explanation string `json:"explanation"`
		Lang        string `json:"lang"`
		SpaceID     string `json:"spaceId"`
	}
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Topic) == "" || strings.TrimSpace(in.Explanation) == "" {
		writeErr(w, http.StatusBadRequest, errors.New("topic and explanation are required"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, fmt.Errorf("streaming not supported"))
		return
	}
	sseHeaders(w)

	report, err := a.llm.Stream(ctx, withLang(feynmanSystem, in.Lang), feynmanPrompt(in.Topic, in.Explanation, ""), sseToken(w, flusher))
	if err != nil {
		sseError(w, flusher, err)
		return
	}
	anns := a.annotateExplanation(ctx, in.Explanation, report, in.Lang)

	doc := &TutorDoc{
		SchemaVersion: "1.0",
		ID:            newID("doc"),
		Title:         "Feynman: " + deriveTitle(in.Topic),
		SpaceID:       in.SpaceID,
		Mode:          "feynman",
		RootQuestion:  in.Topic,
		CreatedAt:     nowISO(),
		Provider:      Provider{Name: a.llm.Name(), Model: a.llm.Model()},
		Blocks:        append([]Block{feynmanExplanationBlock(in.Explanation, 1, anns)}, parseBlocks(report)...),
		Threads:       []Thread{},
		Links:         []DocLink{},
		Revisions:     []Revision{},
	}
	if err := a.store.Save(doc); err != nil {
		sseError(w, flusher, err)
		return
	}
	fmt.Fprintf(w, "data: [DONE] %s\n\n", doc.ID)
	flusher.Flush()
}

func (a *API) feynmanRound(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Explanation string `json:"explanation"`
		Lang        string `json:"lang"`
	}
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Explanation) == "" {
		writeErr(w, http.StatusBadRequest, errors.New("explanation is required"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	docID := r.PathValue("id")
	doc, err := a.store.Load(docID)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if doc.Mode != "feynman" {
		writeErr(w, http.StatusBadRequest, errors.New("not a Feynman session"))
		return
	}

	round := feynmanRoundCount(doc) + 1
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, fmt.Errorf("streaming not supported"))
		return
	}
	sseHeaders(w)

	report, err := a.llm.Stream(ctx, withLang(feynmanSystem, in.Lang),
		feynmanPrompt(doc.RootQuestion, in.Explanation, sessionText(doc)), sseToken(w, flusher))
	if err != nil {
		sseError(w, flusher, err)
		return
	}
	anns := a.annotateExplanation(ctx, in.Explanation, report, in.Lang)

	newBlocks := append([]Block{feynmanExplanationBlock(in.Explanation, round, anns)}, parseBlocks(report)...)
	if _, err := a.store.mutate(docID, func(d *TutorDoc) error {
		d.Blocks = append(d.Blocks, newBlocks...)
		return nil
	}); err != nil {
		sseError(w, flusher, err)
		return
	}
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// feynmanExplanationBlock renders the learner's words as a labeled callout,
// tagged so the frontend can style it and so we can count passes. Any
// annotations are attached for inline highlighting.
func feynmanExplanationBlock(explanation string, round int, anns []Annotation) Block {
	label := "🧑‍🏫 **Your explanation**"
	if round > 1 {
		label = fmt.Sprintf("🧑‍🏫 **Your explanation — pass %d**", round)
	}
	var b strings.Builder
	b.WriteString("> " + label + "\n>\n")
	for _, line := range strings.Split(strings.TrimSpace(explanation), "\n") {
		b.WriteString("> " + line + "\n")
	}
	meta := map[string]interface{}{"feynman": "explanation", "round": round}
	if len(anns) > 0 {
		meta["annotations"] = anns
	}
	return Block{
		BlockID:  newID("blk"),
		Type:     "callout",
		Markdown: strings.TrimRight(b.String(), "\n"),
		Meta:     meta,
	}
}

const annotateSystem = `You mark up a learner's explanation for a study tool.
Given their explanation and a coach's feedback, return ONLY a JSON array of the
specific phrases in the explanation that need attention. Each item:
  {"quote": "<verbatim substring of the explanation>",
   "kind": "gap" | "jargon" | "shaky",
   "note": "<one short clause: why>"}
Rules:
- "quote" MUST be copied verbatim from the explanation (an exact substring).
- kind: "jargon" = a term used without explaining it; "gap" = vague or missing;
  "shaky" = likely incorrect or imprecise.
- At most 6 items. If nothing stands out, return [].
- Output JSON only — no prose, no code fences.`

// noteLangHint forces only the "note" field's language, leaving the verbatim
// "quote" substrings untouched (a blanket directive would risk translating them
// and breaking the substring match that gates each highlight).
func noteLangHint(lang string) string {
	switch lang {
	case "hu":
		return "\n- Write each \"note\" in Hungarian; keep every \"quote\" verbatim."
	case "en":
		return "\n- Write each \"note\" in English; keep every \"quote\" verbatim."
	default:
		return ""
	}
}

// annotateExplanation asks the model for the specific phrases to highlight. It's
// best-effort (bounded time, lenient parsing); failure just means no inline
// highlights — the prose gap report still stands.
func (a *API) annotateExplanation(ctx context.Context, explanation, report, lang string) []Annotation {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	prompt := "Explanation:\n\"\"\"\n" + explanation + "\n\"\"\"\n\nCoach feedback:\n\"\"\"\n" +
		report + "\n\"\"\"\n\nReturn the JSON array."
	out, err := a.llm.Generate(ctx, annotateSystem+noteLangHint(lang), prompt)
	if err != nil {
		return nil
	}
	return parseAnnotations(out, explanation)
}

// parseAnnotations extracts the JSON array from a model response and keeps only
// well-formed items whose quote really occurs in the explanation (dropping any
// hallucinated phrases).
func parseAnnotations(out, explanation string) []Annotation {
	i := strings.Index(out, "[")
	j := strings.LastIndex(out, "]")
	if i < 0 || j <= i {
		return nil
	}
	var raw []Annotation
	if json.Unmarshal([]byte(out[i:j+1]), &raw) != nil {
		return nil
	}
	seen := map[string]bool{}
	anns := []Annotation{}
	for _, an := range raw {
		q := strings.TrimSpace(an.Quote)
		if q == "" || seen[q] || !strings.Contains(explanation, q) {
			continue
		}
		switch an.Kind {
		case "gap", "jargon", "shaky":
		default:
			an.Kind = "gap"
		}
		an.Quote = q
		anns = append(anns, an)
		seen[q] = true
		if len(anns) >= 6 {
			break
		}
	}
	return anns
}

func feynmanRoundCount(doc *TutorDoc) int {
	n := 0
	for _, b := range doc.Blocks {
		if b.Meta != nil && b.Meta["feynman"] == "explanation" {
			n++
		}
	}
	return n
}

// sessionText flattens the session so far into prompt context, keeping the tail
// if it grows large (the recent passes matter most).
func sessionText(doc *TutorDoc) string {
	var b strings.Builder
	for _, blk := range doc.Blocks {
		b.WriteString(blk.Markdown)
		b.WriteString("\n\n")
	}
	s := strings.TrimSpace(b.String())
	const max = 6000
	if len(s) > max {
		s = "…" + s[len(s)-max:]
	}
	return s
}
