package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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
