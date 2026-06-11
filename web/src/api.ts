import type { Anchor, DocSummary, TutorDoc } from "./types";

// In dev (Vite proxy) BASE is empty so /api/... works unchanged.
// In a packaged Electron build VITE_API_BASE is set to http://localhost:PORT.
const BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? "";

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}/api${path}`, {
    headers: { "Content-Type": "application/json" },
    ...init,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as { error?: string }).error || `HTTP ${res.status}`);
  }
  if (res.status === 204) return undefined as T;
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

  importDoc: (markdown: string, title?: string) =>
    req<TutorDoc>("/documents", {
      method: "POST",
      body: JSON.stringify({ markdown, title: title ?? "" }),
    }),

  deleteDoc: (id: string) =>
    req<void>(`/documents/${id}`, { method: "DELETE" }),

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
