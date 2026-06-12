package main

import "testing"

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
