package main

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestAPI(t *testing.T) *API {
	t.Helper()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &API{store: store, llm: &mockLLM{}}
}

func TestExplainStream(t *testing.T) {
	a := newTestAPI(t)
	body := strings.NewReader(`{"source":"Feature: user can reset password\n  Scenario: forgot password","lang":"en"}`)
	req := httptest.NewRequest("POST", "/api/explain/stream", body)
	rec := httptest.NewRecorder()

	a.explainStream(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	// The stream terminates with "[DONE] <doc-id>" — pull that id out.
	var docID string
	sc := bufio.NewScanner(strings.NewReader(rec.Body.String()))
	for sc.Scan() {
		line := strings.TrimPrefix(sc.Text(), "data: ")
		if strings.HasPrefix(line, "[DONE] ") {
			docID = strings.TrimSpace(strings.TrimPrefix(line, "[DONE] "))
		}
	}
	if docID == "" {
		t.Fatalf("no [DONE] <doc-id> in stream:\n%s", rec.Body.String())
	}

	doc, err := a.store.Load(docID)
	if err != nil {
		t.Fatalf("load created doc: %v", err)
	}
	if len(doc.Blocks) == 0 {
		t.Error("expected generated markdown parsed into blocks")
	}
	if doc.RootQuestion != "" {
		t.Errorf("RootQuestion = %q, want empty (source discarded)", doc.RootQuestion)
	}
	if doc.Mode != "" {
		t.Errorf("Mode = %q, want empty (ordinary doc)", doc.Mode)
	}
	// mockDoc opens with a level-2 heading; the title must be derived from it.
	if doc.Title == "" || doc.Title == "Untitled" {
		t.Errorf("Title = %q, want a heading-derived title", doc.Title)
	}
}

func TestExplainSyncEmptySource(t *testing.T) {
	a := newTestAPI(t)
	req := httptest.NewRequest("POST", "/api/explain", strings.NewReader(`{"source":"   "}`))
	rec := httptest.NewRecorder()

	a.explainSync(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestExplainSync(t *testing.T) {
	a := newTestAPI(t)
	req := httptest.NewRequest("POST", "/api/explain", strings.NewReader(`{"source":"some notes about velocity","lang":"en"}`))
	rec := httptest.NewRecorder()

	a.explainSync(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	var doc TutorDoc
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(doc.Blocks) == 0 {
		t.Error("expected blocks")
	}
	if doc.RootQuestion != "" {
		t.Errorf("RootQuestion = %q, want empty", doc.RootQuestion)
	}
}

func TestTitleFromMarkdownH2(t *testing.T) {
	if got := titleFromMarkdown("## Acceleration\n\nbody"); got != "Acceleration" {
		t.Errorf("H2 title = %q, want Acceleration", got)
	}
	if got := titleFromMarkdown("# Imported\n\nbody"); got != "Imported" {
		t.Errorf("H1 title = %q, want Imported", got)
	}
	if got := titleFromMarkdown("no heading here"); got != "" {
		t.Errorf("no heading = %q, want empty", got)
	}
}
