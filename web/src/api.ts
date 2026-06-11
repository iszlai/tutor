import type { Anchor, DocSummary, TutorDoc } from "./types";

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`/api${path}`, {
    headers: { "Content-Type": "application/json" },
    ...init,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as { error?: string }).error || `HTTP ${res.status}`);
  }
  return res.json() as Promise<T>;
}

export const api = {
  health: () => req<{ provider: string; model: string }>("/health"),

  listDocs: () => req<DocSummary[]>("/documents"),

  getDoc: (id: string) => req<TutorDoc>(`/documents/${id}`),

  createDoc: (question: string) =>
    req<TutorDoc>("/documents", {
      method: "POST",
      body: JSON.stringify({ question }),
    }),

  createThread: (docId: string, anchor: Anchor, message: string) =>
    req<TutorDoc>(`/documents/${docId}/threads`, {
      method: "POST",
      body: JSON.stringify({ anchor, message }),
    }),

  reply: (docId: string, threadId: string, text: string) =>
    req<TutorDoc>(`/documents/${docId}/threads/${threadId}/messages`, {
      method: "POST",
      body: JSON.stringify({ text }),
    }),

  action: (docId: string, threadId: string, type: string) =>
    req<TutorDoc>(`/documents/${docId}/threads/${threadId}/actions`, {
      method: "POST",
      body: JSON.stringify({ type }),
    }),
};
