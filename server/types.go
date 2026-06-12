package main

// Canonical document model. The JSON is the structural source of truth;
// Markdown is regenerated from Blocks on every save. See docs/PLAN.md §4.

type Block struct {
	BlockID  string                 `json:"blockId"`
	Type     string                 `json:"type"` // heading|paragraph|formula|code|table|list|callout|image
	Markdown string                 `json:"markdown"`
	Meta     map[string]interface{} `json:"meta,omitempty"`
}

// Anchor uses three redundant locators, resolved in priority order:
// block id + char offset, falling back to quote+context. See docs/PLAN.md §3.
type Anchor struct {
	StartBlockID string `json:"startBlockId"`
	EndBlockID   string `json:"endBlockId"`
	StartOffset  int    `json:"startOffset"`
	EndOffset    int    `json:"endOffset"`
	ExactQuote   string `json:"exactQuote"`
	Prefix       string `json:"prefix"`
	Suffix       string `json:"suffix"`
	Orphaned     bool   `json:"orphaned"`
}

type Message struct {
	ID        string `json:"id"`
	Role      string `json:"role"` // user|assistant
	Text      string `json:"text"`
	CreatedAt string `json:"createdAt"`
	Model     string `json:"model,omitempty"`
}

type ThreadAction struct {
	Type         string `json:"type"`
	CreatedAt    string `json:"createdAt"`
	TargetDocID  string `json:"targetDocId,omitempty"`
	TargetBlock  string `json:"targetBlockId,omitempty"`
	RevisionID   string `json:"revisionId,omitempty"`
}

type Thread struct {
	ThreadID  string         `json:"threadId"`
	Anchor    Anchor         `json:"anchor"`
	Status    string         `json:"status"` // open|resolved
	CreatedAt string         `json:"createdAt"`
	Messages  []Message      `json:"messages"`
	Actions   []ThreadAction `json:"actions"`
}

type DocLink struct {
	Type          string `json:"type"` // child|related
	TargetDocID   string `json:"targetDocId"`
	Label         string `json:"label"`
	SourceThread  string `json:"sourceThreadId,omitempty"`
	SourceBlock   string `json:"sourceBlockId,omitempty"`
}

type Revision struct {
	RevisionID string `json:"revisionId"`
	At         string `json:"at"`
	Kind       string `json:"kind"` // blockRewrite|blockInsert
	BlockID    string `json:"blockId"`
	Before     string `json:"before,omitempty"`
	After      string `json:"after"`
	ByThreadID string `json:"byThreadId,omitempty"`
}

type Provider struct {
	Name  string `json:"name"`
	Model string `json:"model"`
}

type TutorDoc struct {
	SchemaVersion string     `json:"schemaVersion"`
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	SpaceID       string     `json:"spaceId,omitempty"` // "" = no space (unfiled)
	Mode          string     `json:"mode,omitempty"`    // "" (normal) | "feynman"
	RootQuestion  string     `json:"rootQuestion"`
	CreatedAt     string     `json:"createdAt"`
	UpdatedAt     string     `json:"updatedAt"`
	Provider      Provider   `json:"provider"`
	Blocks        []Block    `json:"blocks"`
	Threads       []Thread   `json:"threads"`
	Links         []DocLink  `json:"links"`
	Revisions     []Revision `json:"revisions"`
}
