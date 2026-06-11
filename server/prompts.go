package main

import (
	"fmt"
	"strings"
)

const docSystem = `You are Tutor, an expert explainer. Produce a concise, well-structured ` +
	`ONE-PAGE explanation of the user's question in GitHub-flavored Markdown.

Rules:
- Open with a level-2 heading naming the topic.
- Use short paragraphs, headings, and bullet lists. Keep it to roughly one screen.
- Put math in KaTeX: inline as $...$ and display as $$...$$.
- Use fenced code blocks where code or worked steps help.
- Define symbols you introduce.
- Output Markdown only — no preamble, no closing remarks.`

const replySystem = `You are Tutor, replying inside a comment thread attached to a specific ` +
	`selection of a learning document. Answer the user's question about that selection ` +
	`concisely (1-3 short paragraphs). Use KaTeX for math. Output Markdown only.`

func docPrompt(question string) string {
	return "Explain this for a curious learner:\n\n" + question
}

// threadReplyPrompt gives the model the document context, the selected span,
// and the conversation so far. We send only nearby context, not the whole doc.
func threadReplyPrompt(doc *TutorDoc, t *Thread) string {
	var b strings.Builder
	b.WriteString("Document title: " + doc.Title + "\n\n")
	b.WriteString("Selected text: " + t.Anchor.ExactQuote + "\n\n")

	if blk := findBlock(doc, t.Anchor.StartBlockID); blk != nil {
		b.WriteString("From this passage:\n\"\"\"\n" + blk.Markdown + "\n\"\"\"\n\n")
	}

	b.WriteString("Thread so far:\n")
	for _, m := range t.Messages {
		b.WriteString(fmt.Sprintf("- %s: %s\n", m.Role, m.Text))
	}
	b.WriteString("\nReply to the latest user message.")
	return b.String()
}

func linkedPagePrompt(t *Thread) string {
	return docPrompt("the concept of \"" + t.Anchor.ExactQuote +
		"\", expanded into its own one-page explanation")
}

func summaryPrompt(t *Thread) string {
	var b strings.Builder
	b.WriteString("Summarize this discussion in 1-2 sentences of Markdown, " +
		"suitable to insert into a document as a clarifying note:\n\n")
	for _, m := range t.Messages {
		b.WriteString(fmt.Sprintf("- %s: %s\n", m.Role, m.Text))
	}
	return b.String()
}

func findBlock(doc *TutorDoc, id string) *Block {
	for i := range doc.Blocks {
		if doc.Blocks[i].BlockID == id {
			return &doc.Blocks[i]
		}
	}
	return nil
}
