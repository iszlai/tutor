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

// Explain mode: the user pastes a document (a feature file, spec, one-pager,
// notes) and Tutor generates a one-page explanation from it, adapting its
// treatment to the input. The pasted source is discarded — only the generated
// explanation is kept, as an ordinary TutorDoc (empty Mode), so commenting,
// rewrite, linked pages, etc. all work on the result for free.

// maxSource bounds the pasted document before it reaches the model. Whole
// documents can be large (more generous than Feynman's 6000-char session
// bound). We keep the head — the beginning of a document is its most
// representative part — unlike sessionText, which keeps the tail.
const maxSource = 20000

func clampSource(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > maxSource {
		s = s[:maxSource] + "…"
	}
	return s
}

func (a *API) explainStream(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Source string `json:"source"`
		Lang   string `json:"lang"`
	}
	if !decode(w, r, &in) {
		return
	}
	source := clampSource(in.Source)
	if source == "" {
		writeErr(w, http.StatusBadRequest, errors.New("source is required"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, fmt.Errorf("streaming not supported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	md, err := a.llm.Stream(ctx, withLang(explainSystem, in.Lang), explainPrompt(source), func(token string) {
		b, _ := json.Marshal(token)
		fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
	})
	if err != nil {
		fmt.Fprintf(w, "data: [ERROR] %s\n\n", err.Error())
		flusher.Flush()
		return
	}

	doc := explainDoc(a, md)
	if err := a.store.Save(doc); err != nil {
		fmt.Fprintf(w, "data: [ERROR] %s\n\n", err.Error())
		flusher.Flush()
		return
	}

	fmt.Fprintf(w, "data: [DONE] %s\n\n", doc.ID)
	flusher.Flush()
}

func (a *API) explainSync(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Source string `json:"source"`
		Lang   string `json:"lang"`
	}
	if !decode(w, r, &in) {
		return
	}
	source := clampSource(in.Source)
	if source == "" {
		writeErr(w, http.StatusBadRequest, errors.New("source is required"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	md, err := a.llm.Generate(ctx, withLang(explainSystem, in.Lang), explainPrompt(source))
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}

	doc := explainDoc(a, md)
	if err := a.store.Save(doc); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, doc)
}

// explainDoc builds the TutorDoc from generated markdown. RootQuestion is left
// empty (the source is discarded); the title comes from the generated heading.
func explainDoc(a *API, md string) *TutorDoc {
	title := titleFromMarkdown(md)
	if title == "" {
		title = "Untitled"
	}
	return &TutorDoc{
		SchemaVersion: "1.0",
		ID:            newID("doc"),
		Title:         title,
		CreatedAt:     nowISO(),
		Provider:      Provider{Name: a.llm.Name(), Model: a.llm.Model()},
		Blocks:        parseBlocks(md),
		Threads:       []Thread{},
		Links:         []DocLink{},
		Revisions:     []Revision{},
	}
}
