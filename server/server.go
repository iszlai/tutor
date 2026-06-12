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

type API struct {
	store *Store
	llm   LLM
}

func (a *API) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", a.health)
	mux.HandleFunc("GET /api/documents", a.listDocs)
	mux.HandleFunc("GET /api/search", a.search)
	mux.HandleFunc("POST /api/documents", a.createDoc)
	mux.HandleFunc("POST /api/documents/stream", a.createDocStream)
	mux.HandleFunc("POST /api/feynman", a.createFeynman)
	mux.HandleFunc("POST /api/documents/{id}/feynman", a.feynmanRound)
	mux.HandleFunc("GET /api/documents/{id}", a.getDoc)
	mux.HandleFunc("DELETE /api/documents/{id}", a.deleteDoc)
	mux.HandleFunc("POST /api/documents/{id}/threads", a.createThread)
	mux.HandleFunc("POST /api/documents/{id}/threads/{threadId}/messages", a.replyThread)
	mux.HandleFunc("POST /api/documents/{id}/threads/{threadId}/actions", a.threadAction)
	return withCORS(mux)
}

func (a *API) deleteDoc(w http.ResponseWriter, r *http.Request) {
	if err := a.store.Delete(r.PathValue("id")); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok", "provider": a.llm.Name(), "model": a.llm.Model(),
	})
}

func (a *API) listDocs(w http.ResponseWriter, _ *http.Request) {
	docs, err := a.store.List()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, docs)
}

func (a *API) search(w http.ResponseWriter, r *http.Request) {
	hits, err := a.store.Search(r.URL.Query().Get("q"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, hits)
}

func (a *API) getDoc(w http.ResponseWriter, r *http.Request) {
	doc, err := a.store.Load(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (a *API) createDoc(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Question string `json:"question"`
		Markdown string `json:"markdown"` // optional: import raw md instead of generating
		Title    string `json:"title"`    // optional: override title when importing
		Lang     string `json:"lang"`     // "en" | "hu": forces the response language
	}
	if !decode(w, r, &in) {
		return
	}

	var md, title string

	if in.Markdown != "" {
		// Import path — no LLM call.
		md = in.Markdown
		title = in.Title
		if title == "" {
			title = titleFromMarkdown(md)
		}
		if title == "" {
			title = "Untitled"
		}
	} else {
		// Generate path.
		if strings.TrimSpace(in.Question) == "" {
			writeErr(w, http.StatusBadRequest, errors.New("question is required"))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()
		var err error
		md, err = a.llm.Generate(ctx, withLang(docSystem, in.Lang), docPrompt(in.Question))
		if err != nil {
			writeErr(w, http.StatusBadGateway, err)
			return
		}
		title = deriveTitle(in.Question)
	}

	doc := &TutorDoc{
		SchemaVersion: "1.0",
		ID:            newID("doc"),
		Title:         title,
		RootQuestion:  in.Question,
		CreatedAt:     nowISO(),
		Provider:      Provider{Name: a.llm.Name(), Model: a.llm.Model()},
		Blocks:        parseBlocks(md),
		Threads:       []Thread{},
		Links:         []DocLink{},
		Revisions:     []Revision{},
	}
	if err := a.store.Save(doc); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, doc)
}

func (a *API) createDocStream(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Question string `json:"question"`
		Lang     string `json:"lang"`
	}
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Question) == "" {
		writeErr(w, http.StatusBadRequest, errors.New("question is required"))
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

	md, err := a.llm.Stream(ctx, withLang(docSystem, in.Lang), docPrompt(in.Question), func(token string) {
		b, _ := json.Marshal(token)
		fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
	})
	if err != nil {
		fmt.Fprintf(w, "data: [ERROR] %s\n\n", err.Error())
		flusher.Flush()
		return
	}

	doc := &TutorDoc{
		SchemaVersion: "1.0",
		ID:            newID("doc"),
		Title:         deriveTitle(in.Question),
		RootQuestion:  in.Question,
		CreatedAt:     nowISO(),
		Provider:      Provider{Name: a.llm.Name(), Model: a.llm.Model()},
		Blocks:        parseBlocks(md),
		Threads:       []Thread{},
		Links:         []DocLink{},
		Revisions:     []Revision{},
	}
	if err := a.store.Save(doc); err != nil {
		fmt.Fprintf(w, "data: [ERROR] %s\n\n", err.Error())
		flusher.Flush()
		return
	}

	fmt.Fprintf(w, "data: [DONE] %s\n\n", doc.ID)
	flusher.Flush()
}

func (a *API) createThread(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Anchor  Anchor `json:"anchor"`
		Message string `json:"message"`
		Lang    string `json:"lang"`
	}
	if !decode(w, r, &in) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	doc, err := a.store.Load(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}

	t := Thread{
		ThreadID:  newID("thr"),
		Anchor:    in.Anchor,
		Status:    "open",
		CreatedAt: nowISO(),
		Messages: []Message{{
			ID: newID("msg"), Role: "user", Text: in.Message, CreatedAt: nowISO(),
		}},
		Actions: []ThreadAction{},
	}

	reply, err := a.llm.Generate(ctx, withLang(replySystem, in.Lang), threadReplyPrompt(doc, &t))
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	t.Messages = append(t.Messages, Message{
		ID: newID("msg"), Role: "assistant", Text: reply,
		CreatedAt: nowISO(), Model: a.llm.Model(),
	})
	doc.Threads = append(doc.Threads, t)

	if err := a.store.Save(doc); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, doc)
}

func (a *API) replyThread(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Text string `json:"text"`
		Lang string `json:"lang"`
	}
	if !decode(w, r, &in) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	docID := r.PathValue("id")
	threadID := r.PathValue("threadId")

	// Load doc snapshot to build prompt (not saved until stream completes).
	doc, err := a.store.Load(docID)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	t := findThread(doc, threadID)
	if t == nil {
		writeErr(w, http.StatusNotFound, errNotFound)
		return
	}
	userMsg := Message{ID: newID("msg"), Role: "user", Text: in.Text, CreatedAt: nowISO()}
	t.Messages = append(t.Messages, userMsg)
	prompt := threadReplyPrompt(doc, t)

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, fmt.Errorf("streaming not supported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	reply, streamErr := a.llm.Stream(ctx, withLang(replySystem, in.Lang), prompt, func(token string) {
		b, _ := json.Marshal(token)
		fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
	})

	if streamErr != nil {
		fmt.Fprintf(w, "data: [ERROR] %s\n\n", streamErr.Error())
		flusher.Flush()
		return
	}

	// Persist both messages atomically after the stream completes.
	assistantMsg := Message{
		ID: newID("msg"), Role: "assistant", Text: reply,
		CreatedAt: nowISO(), Model: a.llm.Model(),
	}
	if _, err := a.store.mutate(docID, func(d *TutorDoc) error {
		th := findThread(d, threadID)
		if th == nil {
			return errNotFound
		}
		th.Messages = append(th.Messages, userMsg, assistantMsg)
		return nil
	}); err != nil {
		fmt.Fprintf(w, "data: [ERROR] %s\n\n", err.Error())
		flusher.Flush()
		return
	}

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (a *API) threadAction(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Type string `json:"type"`
		Lang string `json:"lang"`
	}
	if !decode(w, r, &in) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	doc, err := a.runAction(ctx, r.PathValue("id"), r.PathValue("threadId"), in.Type, in.Lang)
	if err != nil {
		writeErr(w, statusFor(err), err)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}
