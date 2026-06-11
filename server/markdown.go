package main

import (
	"regexp"
	"strings"
)

// parseBlocks splits a Markdown document into top-level blocks and assigns a
// stable blockId to each. Splitting is by blank lines, with fenced code blocks
// kept intact. This is deliberately simple — the canonical structure lives in
// JSON; this just gives us addressable units to anchor comments against.
func parseBlocks(md string) []Block {
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")

	var blocks []Block
	var cur []string
	inFence := false

	flush := func() {
		text := strings.TrimRight(strings.Join(cur, "\n"), "\n")
		cur = nil
		if strings.TrimSpace(text) == "" {
			return
		}
		blocks = append(blocks, Block{
			BlockID:  newID("blk"),
			Type:     classify(text),
			Markdown: text,
			Meta:     metaFor(text),
		})
	}

	for _, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if strings.HasPrefix(trimmed, "```") {
			cur = append(cur, ln)
			if inFence {
				inFence = false
				flush()
			} else {
				inFence = true
			}
			continue
		}
		if inFence {
			cur = append(cur, ln)
			continue
		}
		if trimmed == "" {
			flush()
			continue
		}
		cur = append(cur, ln)
	}
	flush()
	return blocks
}

var (
	headingRe = regexp.MustCompile(`^#{1,6}\s`)
	listRe    = regexp.MustCompile(`(?m)^\s*([-*+]|\d+\.)\s`)
	tableRe   = regexp.MustCompile(`(?m)^\s*\|.*\|\s*$`)
)

func classify(text string) string {
	t := strings.TrimSpace(text)
	switch {
	case strings.HasPrefix(t, "```"):
		return "code"
	case headingRe.MatchString(t):
		return "heading"
	case strings.HasPrefix(t, "$$") && strings.HasSuffix(t, "$$"):
		return "formula"
	case strings.HasPrefix(t, "> "):
		return "callout"
	case tableRe.MatchString(t):
		return "table"
	case listRe.MatchString(t):
		return "list"
	default:
		return "paragraph"
	}
}

func metaFor(text string) map[string]interface{} {
	t := strings.TrimSpace(text)
	if strings.HasPrefix(t, "```") {
		first := strings.SplitN(t, "\n", 2)[0]
		lang := strings.TrimSpace(strings.TrimPrefix(first, "```"))
		if lang != "" {
			return map[string]interface{}{"lang": lang}
		}
	}
	if headingRe.MatchString(t) {
		level := 0
		for _, c := range t {
			if c == '#' {
				level++
			} else {
				break
			}
		}
		return map[string]interface{}{"level": level}
	}
	return nil
}

// renderMarkdown regenerates the human-readable Markdown file from blocks.
// Block ids are embedded as HTML comments so they round-trip on import without
// polluting the rendered output. See docs/PLAN.md §4.1.
func renderMarkdown(doc *TutorDoc) string {
	var b strings.Builder
	b.WriteString("# " + doc.Title + "\n\n")
	for _, blk := range doc.Blocks {
		b.WriteString("<!-- blk:" + blk.BlockID + " -->\n")
		b.WriteString(blk.Markdown)
		b.WriteString("\n\n")
	}
	return b.String()
}
