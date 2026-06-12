package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Feynman mode: the learner teaches a concept back in their own words and Tutor
// returns a supportive gap report. A session is a TutorDoc with Mode "feynman";
// each pass appends the learner's explanation (a callout) followed by the gap
// report, so the whole loop is just blocks — search, rendering, and persistence
// come for free.

func (a *API) createFeynman(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Topic       string `json:"topic"`
		Explanation string `json:"explanation"`
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

	report, err := a.llm.Stream(ctx, feynmanSystem, feynmanPrompt(in.Topic, in.Explanation, ""), sseToken(w, flusher))
	if err != nil {
		sseError(w, flusher, err)
		return
	}

	doc := &TutorDoc{
		SchemaVersion: "1.0",
		ID:            newID("doc"),
		Title:         "Feynman: " + deriveTitle(in.Topic),
		Mode:          "feynman",
		RootQuestion:  in.Topic,
		CreatedAt:     nowISO(),
		Provider:      Provider{Name: a.llm.Name(), Model: a.llm.Model()},
		Blocks:        append([]Block{feynmanExplanationBlock(in.Explanation, 1)}, parseBlocks(report)...),
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

	report, err := a.llm.Stream(ctx, feynmanSystem,
		feynmanPrompt(doc.RootQuestion, in.Explanation, sessionText(doc)), sseToken(w, flusher))
	if err != nil {
		sseError(w, flusher, err)
		return
	}

	newBlocks := append([]Block{feynmanExplanationBlock(in.Explanation, round)}, parseBlocks(report)...)
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
// tagged so the frontend can style it and so we can count passes.
func feynmanExplanationBlock(explanation string, round int) Block {
	label := "🧑‍🏫 **Your explanation**"
	if round > 1 {
		label = fmt.Sprintf("🧑‍🏫 **Your explanation — pass %d**", round)
	}
	var b strings.Builder
	b.WriteString("> " + label + "\n>\n")
	for _, line := range strings.Split(strings.TrimSpace(explanation), "\n") {
		b.WriteString("> " + line + "\n")
	}
	return Block{
		BlockID:  newID("blk"),
		Type:     "callout",
		Markdown: strings.TrimRight(b.String(), "\n"),
		Meta:     map[string]interface{}{"feynman": "explanation", "round": round},
	}
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
