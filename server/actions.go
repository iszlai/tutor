package main

import (
	"context"
	"errors"
)

var (
	errNotFound  = errors.New("not found")
	errBadAction = errors.New("unknown action type")
)

// runAction evolves the document from a thread's discussion. Each action records
// a revision and/or a link so the change is reconstructable. See docs/PLAN.md §6.
func (a *API) runAction(ctx context.Context, docID, threadID, action string) (*TutorDoc, error) {
	if action == "createLinkedPage" {
		return a.createLinkedPage(ctx, docID, threadID)
	}

	return a.store.mutate(docID, func(doc *TutorDoc) error {
		t := findThread(doc, threadID)
		if t == nil {
			return errNotFound
		}
		switch action {
		case "insertSummary":
			return a.insertSummary(ctx, doc, t)
		case "rewrite":
			return a.rewriteBlock(ctx, doc, t)
		case "generateVisual":
			return a.generateVisual(ctx, doc, t)
		case "generateExercise":
			return a.generateExercise(ctx, doc, t)
		default:
			return errBadAction
		}
	})
}

func (a *API) insertSummary(ctx context.Context, doc *TutorDoc, t *Thread) error {
	summary, err := a.llm.Generate(ctx, replySystem, summaryPrompt(t))
	if err != nil {
		return err
	}
	blk := Block{BlockID: newID("blk"), Type: "callout", Markdown: "> " + summary}
	insertAfter(doc, t.Anchor.StartBlockID, blk)
	doc.Revisions = append(doc.Revisions, Revision{
		RevisionID: newID("rev"), At: nowISO(), Kind: "blockInsert",
		BlockID: blk.BlockID, After: blk.Markdown, ByThreadID: t.ThreadID,
	})
	t.Actions = append(t.Actions, ThreadAction{
		Type: "insertSummary", CreatedAt: nowISO(), TargetBlock: blk.BlockID,
	})
	return nil
}

func (a *API) rewriteBlock(ctx context.Context, doc *TutorDoc, t *Thread) error {
	blk := findBlock(doc, t.Anchor.StartBlockID)
	if blk == nil {
		return errNotFound
	}
	prompt := "Rewrite the following passage to be clearer and simpler, " +
		"incorporating the discussion below. Output Markdown only.\n\nPassage:\n\"\"\"\n" +
		blk.Markdown + "\n\"\"\"\n\n" + summaryPrompt(t)
	rewritten, err := a.llm.Generate(ctx, replySystem, prompt)
	if err != nil {
		return err
	}
	rev := Revision{
		RevisionID: newID("rev"), At: nowISO(), Kind: "blockRewrite",
		BlockID: blk.BlockID, Before: blk.Markdown, After: rewritten, ByThreadID: t.ThreadID,
	}
	blk.Markdown = rewritten
	doc.Revisions = append(doc.Revisions, rev)
	t.Actions = append(t.Actions, ThreadAction{
		Type: "rewrite", CreatedAt: nowISO(), TargetBlock: blk.BlockID, RevisionID: rev.RevisionID,
	})
	return nil
}

func (a *API) generateVisual(ctx context.Context, doc *TutorDoc, t *Thread) error {
	prompt := "Produce a Mermaid diagram that helps explain \"" + t.Anchor.ExactQuote +
		"\". Output only a single ```mermaid fenced code block."
	out, err := a.llm.Generate(ctx, replySystem, prompt)
	if err != nil {
		return err
	}
	blk := Block{BlockID: newID("blk"), Type: "code", Markdown: out, Meta: map[string]interface{}{"lang": "mermaid"}}
	insertAfter(doc, t.Anchor.StartBlockID, blk)
	doc.Revisions = append(doc.Revisions, Revision{
		RevisionID: newID("rev"), At: nowISO(), Kind: "blockInsert",
		BlockID: blk.BlockID, After: blk.Markdown, ByThreadID: t.ThreadID,
	})
	t.Actions = append(t.Actions, ThreadAction{
		Type: "generateVisual", CreatedAt: nowISO(), TargetBlock: blk.BlockID,
	})
	return nil
}

func (a *API) generateExercise(ctx context.Context, doc *TutorDoc, t *Thread) error {
	prompt := "Create one short worked example and one practice question about \"" +
		t.Anchor.ExactQuote + "\". Use a level-3 heading 'Practice'. Output Markdown only."
	out, err := a.llm.Generate(ctx, replySystem, prompt)
	if err != nil {
		return err
	}
	blk := Block{BlockID: newID("blk"), Type: "paragraph", Markdown: out}
	insertAfter(doc, t.Anchor.StartBlockID, blk)
	doc.Revisions = append(doc.Revisions, Revision{
		RevisionID: newID("rev"), At: nowISO(), Kind: "blockInsert",
		BlockID: blk.BlockID, After: blk.Markdown, ByThreadID: t.ThreadID,
	})
	t.Actions = append(t.Actions, ThreadAction{
		Type: "generateExercise", CreatedAt: nowISO(), TargetBlock: blk.BlockID,
	})
	return nil
}

// createLinkedPage spawns a child document and links it from the parent thread.
func (a *API) createLinkedPage(ctx context.Context, docID, threadID string) (*TutorDoc, error) {
	parent, err := a.store.Load(docID)
	if err != nil {
		return nil, errNotFound
	}
	t := findThread(parent, threadID)
	if t == nil {
		return nil, errNotFound
	}

	md, err := a.llm.Generate(ctx, docSystem, linkedPagePrompt(t))
	if err != nil {
		return nil, err
	}
	child := &TutorDoc{
		SchemaVersion: "1.0",
		ID:            newID("doc"),
		Title:        deriveTitle(t.Anchor.ExactQuote),
		RootQuestion: "Linked from: " + parent.Title,
		CreatedAt:    nowISO(),
		Provider:     Provider{Name: a.llm.Name(), Model: a.llm.Model()},
		Blocks:       parseBlocks(md),
		Threads:      []Thread{},
		Links:        []DocLink{{Type: "related", TargetDocID: parent.ID, Label: parent.Title}},
		Revisions:    []Revision{},
	}
	if err := a.store.Save(child); err != nil {
		return nil, err
	}

	// Record the link + action on the parent and return the parent.
	return a.store.mutate(docID, func(doc *TutorDoc) error {
		doc.Links = append(doc.Links, DocLink{
			Type: "child", TargetDocID: child.ID, Label: child.Title,
			SourceThread: threadID, SourceBlock: t.Anchor.StartBlockID,
		})
		if pt := findThread(doc, threadID); pt != nil {
			pt.Actions = append(pt.Actions, ThreadAction{
				Type: "createLinkedPage", CreatedAt: nowISO(), TargetDocID: child.ID,
			})
		}
		return nil
	})
}

// insertAfter places blk immediately after the block with id `afterID`
// (or at the end if not found).
func insertAfter(doc *TutorDoc, afterID string, blk Block) {
	for i := range doc.Blocks {
		if doc.Blocks[i].BlockID == afterID {
			doc.Blocks = append(doc.Blocks[:i+1], append([]Block{blk}, doc.Blocks[i+1:]...)...)
			return
		}
	}
	doc.Blocks = append(doc.Blocks, blk)
}

func findThread(doc *TutorDoc, id string) *Thread {
	for i := range doc.Threads {
		if doc.Threads[i].ThreadID == id {
			return &doc.Threads[i]
		}
	}
	return nil
}
