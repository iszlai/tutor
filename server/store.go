package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	_ "modernc.org/sqlite"
)

// Store persists documents and spaces in a single SQLite database file
// (tutor.db, under the data dir). The full TutorDoc is kept as a JSON blob —
// the structural source of truth — while a few columns (title, timestamps,
// space) are denormalized so listing, filtering, and ordering don't have to
// unmarshal every row. A pure-Go driver (modernc.org/sqlite) is used so the
// server cross-compiles for the Electron build without cgo. See docs/PLAN.md §4.
type Store struct {
	db *sql.DB
	mu sync.Mutex
}

func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "tutor.db")
	dsn := "file:" + path +
		"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	// One-time, idempotent import of any documents left over from the previous
	// file-based store (folders of document.json under the same data dir).
	if n, err := s.importLegacy(dir); err != nil {
		log.Printf("legacy import: %v", err)
	} else if n > 0 {
		log.Printf("imported %d document(s) from the legacy file store", n)
	}
	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS spaces (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS documents (
  id         TEXT PRIMARY KEY,
  title      TEXT NOT NULL,
  space_id   TEXT REFERENCES spaces(id) ON DELETE SET NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  data       TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_documents_space   ON documents(space_id);
CREATE INDEX IF NOT EXISTS idx_documents_updated ON documents(updated_at DESC);
`)
	return err
}

// ---- Documents -------------------------------------------------------------

func (s *Store) Save(doc *TutorDoc) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc.UpdatedAt = nowISO()
	if doc.CreatedAt == "" {
		doc.CreatedAt = doc.UpdatedAt
	}
	data, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
INSERT INTO documents (id, title, space_id, created_at, updated_at, data)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  title      = excluded.title,
  space_id   = excluded.space_id,
  updated_at = excluded.updated_at,
  data       = excluded.data`,
		doc.ID, doc.Title, nullable(doc.SpaceID), doc.CreatedAt, doc.UpdatedAt, string(data))
	return err
}

func (s *Store) Load(id string) (*TutorDoc, error) {
	var data string
	err := s.db.QueryRow(`SELECT data FROM documents WHERE id = ?`, id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	var doc TutorDoc
	if err := json.Unmarshal([]byte(data), &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM documents WHERE id = ?`, id)
	return err
}

type DocSummary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	UpdatedAt string `json:"updatedAt"`
	SpaceID   string `json:"spaceId,omitempty"`
}

// List returns document summaries, newest first. The space argument filters:
//
//	""      → all documents
//	"none"  → only documents not in any space
//	<id>    → only documents in that space
func (s *Store) List(space string) ([]DocSummary, error) {
	const cols = `SELECT id, title, COALESCE(space_id, ''), updated_at FROM documents`
	var (
		rows *sql.Rows
		err  error
	)
	switch space {
	case "":
		rows, err = s.db.Query(cols + ` ORDER BY updated_at DESC`)
	case "none":
		rows, err = s.db.Query(cols + ` WHERE space_id IS NULL ORDER BY updated_at DESC`)
	default:
		rows, err = s.db.Query(cols+` WHERE space_id = ? ORDER BY updated_at DESC`, space)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []DocSummary{}
	for rows.Next() {
		var d DocSummary
		if err := rows.Scan(&d.ID, &d.Title, &d.SpaceID, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// SetDocSpace moves a document into a space ("" to unfile it). A non-empty
// target space must exist.
func (s *Store) SetDocSpace(id, spaceID string) (*TutorDoc, error) {
	if spaceID != "" {
		var x int
		err := s.db.QueryRow(`SELECT 1 FROM spaces WHERE id = ?`, spaceID).Scan(&x)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errNotFound
		}
		if err != nil {
			return nil, err
		}
	}
	return s.mutate(id, func(d *TutorDoc) error {
		d.SpaceID = spaceID
		return nil
	})
}

// ---- Spaces ----------------------------------------------------------------

type Space struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
	DocCount  int    `json:"docCount"`
}

func (s *Store) ListSpaces() ([]Space, error) {
	rows, err := s.db.Query(`
SELECT sp.id, sp.name, sp.created_at, COUNT(d.id)
FROM spaces sp
LEFT JOIN documents d ON d.space_id = sp.id
GROUP BY sp.id, sp.name, sp.created_at
ORDER BY sp.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Space{}
	for rows.Next() {
		var sp Space
		if err := rows.Scan(&sp.ID, &sp.Name, &sp.CreatedAt, &sp.DocCount); err != nil {
			return nil, err
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}

func (s *Store) CreateSpace(name string) (*Space, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("space name is required")
	}
	sp := &Space{ID: newID("spc"), Name: name, CreatedAt: nowISO()}
	if _, err := s.db.Exec(
		`INSERT INTO spaces (id, name, created_at) VALUES (?, ?, ?)`,
		sp.ID, sp.Name, sp.CreatedAt,
	); err != nil {
		return nil, err
	}
	return sp, nil
}

func (s *Store) RenameSpace(id, name string) (*Space, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("space name is required")
	}
	res, err := s.db.Exec(`UPDATE spaces SET name = ? WHERE id = ?`, name, id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, errNotFound
	}
	return &Space{ID: id, Name: name}, nil
}

// DeleteSpace removes a space; its documents are unfiled (ON DELETE SET NULL),
// not deleted.
func (s *Store) DeleteSpace(id string) error {
	res, err := s.db.Exec(`DELETE FROM spaces WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errNotFound
	}
	return nil
}

// ---- Search ----------------------------------------------------------------

type SearchHit struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	UpdatedAt string `json:"updatedAt"`
	Snippet   string `json:"snippet"`
}

// Search scans every document for all whitespace-separated query terms
// (case-insensitive AND match) across the title, block content, and thread
// messages, returning a contextual snippet per hit, newest first. It's a linear
// scan over the JSON blobs — fine at this single-user scale.
func (s *Store) Search(query string) ([]SearchHit, error) {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return []SearchHit{}, nil
	}
	rows, err := s.db.Query(`SELECT data FROM documents ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hits := []SearchHit{}
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var doc TutorDoc
		if json.Unmarshal([]byte(data), &doc) != nil {
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
	return hits, rows.Err()
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

// ---- Internals -------------------------------------------------------------

func (s *Store) mutate(id string, fn func(*TutorDoc) error) (*TutorDoc, error) {
	doc, err := s.Load(id)
	if err != nil {
		return nil, err
	}
	if err := fn(doc); err != nil {
		return nil, err
	}
	if err := s.Save(doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// importLegacy reads any document.json files left by the previous file-based
// store and inserts them into SQLite, preserving their ids and timestamps.
// It's idempotent (INSERT OR IGNORE) and safe to run on every startup.
func (s *Store) importLegacy(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	imported := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name(), "document.json"))
		if err != nil {
			continue
		}
		var doc TutorDoc
		if json.Unmarshal(data, &doc) != nil || doc.ID == "" {
			continue
		}
		updated := doc.UpdatedAt
		if updated == "" {
			updated = doc.CreatedAt
		}
		res, err := s.db.Exec(`
INSERT OR IGNORE INTO documents (id, title, space_id, created_at, updated_at, data)
VALUES (?, ?, NULL, ?, ?, ?)`,
			doc.ID, doc.Title, doc.CreatedAt, updated, string(data))
		if err != nil {
			return imported, err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			imported++
		}
	}
	return imported, nil
}

// nullable maps an empty string to SQL NULL so an unset space_id satisfies the
// foreign-key constraint instead of pointing at a non-existent "" space.
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
