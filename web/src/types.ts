// Mirror of the server's canonical model (server/types.go).

export type BlockType =
  | "heading"
  | "paragraph"
  | "formula"
  | "code"
  | "table"
  | "list"
  | "callout"
  | "image";

export interface Block {
  blockId: string;
  type: BlockType;
  markdown: string;
  meta?: Record<string, unknown>;
}

export interface Anchor {
  startBlockId: string;
  endBlockId: string;
  startOffset: number;
  endOffset: number;
  exactQuote: string;
  prefix: string;
  suffix: string;
  orphaned?: boolean;
}

export interface Message {
  id: string;
  role: "user" | "assistant";
  text: string;
  createdAt: string;
  model?: string;
}

export interface ThreadAction {
  type: string;
  createdAt: string;
  targetDocId?: string;
  targetBlockId?: string;
  revisionId?: string;
}

export interface Thread {
  threadId: string;
  anchor: Anchor;
  status: "open" | "resolved";
  createdAt: string;
  messages: Message[];
  actions: ThreadAction[];
}

export interface DocLink {
  type: "child" | "related";
  targetDocId: string;
  label: string;
  sourceThreadId?: string;
  sourceBlockId?: string;
}

export interface TutorDoc {
  schemaVersion: string;
  id: string;
  title: string;
  mode?: string; // "" (normal) | "feynman"
  rootQuestion: string;
  createdAt: string;
  updatedAt: string;
  provider: { name: string; model: string };
  blocks: Block[];
  threads: Thread[];
  links: DocLink[];
  revisions: unknown[];
}

export interface DocSummary {
  id: string;
  title: string;
  updatedAt: string;
}

export interface SearchHit {
  id: string;
  title: string;
  updatedAt: string;
  snippet: string;
}
