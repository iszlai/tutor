package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

// Store persists each document as a folder containing document.json (canonical)
// and document.md (human-readable). See docs/PLAN.md §4.2.
type Store struct {
	dir string
	mu  sync.Mutex
}

func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

func (s *Store) docDir(id string) string { return filepath.Join(s.dir, id) }

func (s *Store) Save(doc *TutorDoc) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc.UpdatedAt = nowISO()
	dir := s.docDir(doc.ID)
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "document.json"), data, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "document.md"), []byte(renderMarkdown(doc)), 0o644)
}

func (s *Store) Load(id string) (*TutorDoc, error) {
	data, err := os.ReadFile(filepath.Join(s.docDir(id), "document.json"))
	if err != nil {
		return nil, err
	}
	var doc TutorDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

type DocSummary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	UpdatedAt string `json:"updatedAt"`
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.RemoveAll(s.docDir(id))
}

func (s *Store) List() ([]DocSummary, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	out := []DocSummary{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		doc, err := s.Load(e.Name())
		if err != nil {
			continue
		}
		out = append(out, DocSummary{ID: doc.ID, Title: doc.Title, UpdatedAt: doc.UpdatedAt})
	}
	return out, nil
}

type SearchHit struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	UpdatedAt string `json:"updatedAt"`
	Snippet   string `json:"snippet"`
}

// Search scans every document for all whitespace-separated query terms
// (case-insensitive AND match) across the title, block content, and thread
// messages, returning a contextual snippet per hit, newest first. It's a linear
// scan over the file store — fine at this single-user scale.
func (s *Store) Search(query string) ([]SearchHit, error) {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return []SearchHit{}, nil
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	hits := []SearchHit{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		doc, err := s.Load(e.Name())
		if err != nil {
			continue
		}
		var body strings.Builder
		for _, b := range doc.Blocks {
			body.WriteString(b.Markdown)
			body.WriteByte('\n')
		}
		for _, t := range doc.Threads {
			for _, m := range t.Messages {
				body.WriteString(m.Text)
				body.WriteByte('\n')
			}
		}
		if !containsAll(strings.ToLower(doc.Title+"\n"+body.String()), terms) {
			continue
		}
		hits = append(hits, SearchHit{
			ID: doc.ID, Title: doc.Title, UpdatedAt: doc.UpdatedAt,
			Snippet: snippet(body.String(), terms),
		})
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].UpdatedAt > hits[j].UpdatedAt })
	return hits, nil
}

func containsAll(hay string, terms []string) bool {
	for _, t := range terms {
		if !strings.Contains(hay, t) {
			return false
		}
	}
	return true
}

// snippet returns a one-line excerpt of body centered on the earliest matching
// term, with ellipses where it was clipped.
func snippet(body string, terms []string) string {
	flat := strings.Join(strings.Fields(body), " ")
	low := strings.ToLower(flat)
	idx := -1
	for _, t := range terms {
		if i := strings.Index(low, t); i >= 0 && (idx < 0 || i < idx) {
			idx = i
		}
	}
	const radius = 80
	if idx < 0 {
		idx = 0
	}
	start, end := idx-radius, idx+radius
	if start < 0 {
		start = 0
	}
	if end > len(flat) {
		end = len(flat)
	}
	for start > 0 && !utf8.RuneStart(flat[start]) {
		start--
	}
	for end < len(flat) && !utf8.RuneStart(flat[end]) {
		end++
	}
	out := flat[start:end]
	if start > 0 {
		out = "…" + out
	}
	if end < len(flat) {
		out = out + "…"
	}
	return out
}

func (s *Store) mutate(id string, fn func(*TutorDoc) error) (*TutorDoc, error) {
	doc, err := s.Load(id)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", id, err)
	}
	if err := fn(doc); err != nil {
		return nil, err
	}
	if err := s.Save(doc); err != nil {
		return nil, err
	}
	return doc, nil
}
