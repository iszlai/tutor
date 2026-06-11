package main

import (
	"context"
	"errors"
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
	mux.HandleFunc("POST /api/documents", a.createDoc)
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
		md, err = a.llm.Generate(ctx, docSystem, docPrompt(in.Question))
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

func (a *API) createThread(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Anchor  Anchor `json:"anchor"`
		Message string `json:"message"`
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

	reply, err := a.llm.Generate(ctx, replySystem, threadReplyPrompt(doc, &t))
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
	}
	if !decode(w, r, &in) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	threadID := r.PathValue("threadId")
	var reply string
	doc, err := a.store.mutate(r.PathValue("id"), func(doc *TutorDoc) error {
		t := findThread(doc, threadID)
		if t == nil {
			return errNotFound
		}
		t.Messages = append(t.Messages, Message{
			ID: newID("msg"), Role: "user", Text: in.Text, CreatedAt: nowISO(),
		})
		var gerr error
		reply, gerr = a.llm.Generate(ctx, replySystem, threadReplyPrompt(doc, t))
		if gerr != nil {
			return gerr
		}
		t.Messages = append(t.Messages, Message{
			ID: newID("msg"), Role: "assistant", Text: reply,
			CreatedAt: nowISO(), Model: a.llm.Model(),
		})
		return nil
	})
	if err != nil {
		writeErr(w, statusFor(err), err)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (a *API) threadAction(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Type string `json:"type"`
	}
	if !decode(w, r, &in) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	doc, err := a.runAction(ctx, r.PathValue("id"), r.PathValue("threadId"), in.Type)
	if err != nil {
		writeErr(w, statusFor(err), err)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}
