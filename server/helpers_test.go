package main

import (
	"testing"
)

func TestParseAnnotations(t *testing.T) {
	expl := "recursion is when a function calls itself until a base case"

	// Well-formed array, possibly wrapped in prose/fences the model added.
	out := "Sure!\n```json\n[" +
		`{"quote":"calls itself","kind":"gap","note":"how?"},` +
		`{"quote":"base case","kind":"jargon","note":"undefined"}` +
		"]\n```"
	anns := parseAnnotations(out, expl)
	if len(anns) != 2 || anns[0].Quote != "calls itself" || anns[1].Kind != "jargon" {
		t.Fatalf("got %+v", anns)
	}

	// Hallucinated quote (not in explanation) is dropped; bad kind defaults to gap.
	out2 := `[{"quote":"calls itself","kind":"weird","note":"x"},{"quote":"not present","kind":"gap","note":"y"}]`
	anns2 := parseAnnotations(out2, expl)
	if len(anns2) != 1 || anns2[0].Kind != "gap" {
		t.Fatalf("filter/default failed: %+v", anns2)
	}

	// No JSON at all → nil.
	if got := parseAnnotations("nothing structured here", expl); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestStoreSearch(t *testing.T) {
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mk := func(id, title, body string) {
		if err := s.Save(&TutorDoc{
			SchemaVersion: "1.0", ID: id, Title: title,
			Blocks: []Block{{BlockID: "b1", Type: "paragraph", Markdown: body}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	mk("doc_a", "Acceleration", "Acceleration is the rate of change of velocity over time.")
	mk("doc_b", "Photosynthesis", "Plants convert sunlight into chemical energy.")

	// Title match.
	if hits, _ := s.Search("acceleration"); len(hits) != 1 || hits[0].ID != "doc_a" {
		t.Fatalf("title match: got %+v", hits)
	}
	// Body match, case-insensitive, with a snippet.
	hits, _ := s.Search("VELOCITY")
	if len(hits) != 1 || hits[0].ID != "doc_a" {
		t.Fatalf("body match: got %+v", hits)
	}
	if hits[0].Snippet == "" {
		t.Error("expected a snippet for body match")
	}
	// Multi-term AND: both terms must appear in the same doc.
	if hits, _ := s.Search("plants sunlight"); len(hits) != 1 || hits[0].ID != "doc_b" {
		t.Fatalf("AND match: got %+v", hits)
	}
	if hits, _ := s.Search("plants velocity"); len(hits) != 0 {
		t.Fatalf("AND mismatch should be empty: got %+v", hits)
	}
	// Empty query returns nothing.
	if hits, _ := s.Search("   "); len(hits) != 0 {
		t.Fatalf("empty query: got %+v", hits)
	}
}

func TestNormalizeMermaid(t *testing.T) {
	const body = "graph TD\n  A --> B"
	want := "```mermaid\n" + body + "\n```"

	cases := map[string]string{
		"already fenced":   "```mermaid\n" + body + "\n```",
		"bare fence":       "```\n" + body + "\n```",
		"prose around":     "Here is a diagram:\n\n```mermaid\n" + body + "\n```\n\nHope it helps!",
		"raw, no fence":    body,
		"trailing spaces":  "  ```mermaid\n" + body + "\n```  ",
		"missing close":    "```mermaid\n" + body,
	}

	for name, in := range cases {
		if got := normalizeMermaid(in); got != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}
}
