import type { Anchor, DocSummary, SearchHit, TutorDoc } from "./types";
import type { Lang } from "./i18n";

// In dev (Vite proxy) BASE is empty so /api/... works unchanged.
// In a packaged Electron build VITE_API_BASE is set to http://localhost:PORT.
const BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? "";

// Active UI language, sent with every generation request so the server can
// force the AI's response language. App keeps this in sync via api.setLang.
let lang: Lang = "en";

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

// streamSSE POSTs a JSON body and reads a Server-Sent Events stream, calling
// onToken for each text chunk. Returns the payload after a [DONE] marker (e.g. a
// new doc id), or "" if none.
async function streamSSE(
  path: string,
  body: unknown,
  onToken: (t: string) => void
): Promise<string> {
  const res = await fetch(`${BASE}/api${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok || !res.body) throw new Error(`HTTP ${res.status}`);
  const reader = res.body.getReader();
  const dec = new TextDecoder();
  let buf = "";
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    buf += dec.decode(value, { stream: true });
    const parts = buf.split("\n\n");
    buf = parts.pop() ?? "";
    for (const part of parts) {
      if (!part.startsWith("data: ")) continue;
      const data = part.slice(6).trim();
      if (data.startsWith("[DONE]")) return data.slice(6).trim();
      if (data.startsWith("[ERROR]")) throw new Error(data.slice(7).trim() || "stream error");
      try { onToken(JSON.parse(data) as string); } catch { /* skip malformed */ }
    }
  }
  throw new Error("stream closed without [DONE]");
}

export const api = {
  setLang: (l: Lang) => {
    lang = l;
  },

  health: () => req<{ provider: string; model: string }>("/health"),

  listDocs: () => req<DocSummary[]>("/documents"),

  search: (q: string) => req<SearchHit[]>(`/search?q=${encodeURIComponent(q)}`),

  getDoc: (id: string) => req<TutorDoc>(`/documents/${id}`),

  createDoc: (question: string) =>
    req<TutorDoc>("/documents", {
      method: "POST",
      body: JSON.stringify({ question, lang }),
    }),

  createDocStream: async (question: string, onToken: (t: string) => void): Promise<string> => {
    const res = await fetch(`${BASE}/api/documents/stream`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ question, lang }),
    });
    if (!res.ok || !res.body) throw new Error(`HTTP ${res.status}`);
    const reader = res.body.getReader();
    const dec = new TextDecoder();
    let buf = "";
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += dec.decode(value, { stream: true });
      const parts = buf.split("\n\n");
      buf = parts.pop() ?? "";
      for (const part of parts) {
        if (!part.startsWith("data: ")) continue;
        const data = part.slice(6).trim();
        if (data.startsWith("[DONE]")) return data.slice(6).trim(); // doc ID
        if (data.startsWith("[ERROR]")) throw new Error(data.slice(7).trim() || "stream error");
        try { onToken(JSON.parse(data) as string); } catch { /* skip malformed */ }
      }
    }
    throw new Error("stream closed without [DONE]");
  },

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
      body: JSON.stringify({ anchor, message, lang }),
    }),

  reply: (docId: string, threadId: string, text: string) =>
    req<TutorDoc>(`/documents/${docId}/threads/${threadId}/messages`, {
      method: "POST",
      body: JSON.stringify({ text, lang }),
    }),

  replyStream: async (
    docId: string,
    threadId: string,
    text: string,
    onToken: (t: string) => void
  ): Promise<TutorDoc> => {
    const res = await fetch(`${BASE}/api/documents/${docId}/threads/${threadId}/messages`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ text, lang }),
    });
    if (!res.ok || !res.body) throw new Error(`HTTP ${res.status}`);
    const reader = res.body.getReader();
    const dec = new TextDecoder();
    let buf = "";
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += dec.decode(value, { stream: true });
      const parts = buf.split("\n\n");
      buf = parts.pop() ?? "";
      for (const part of parts) {
        if (!part.startsWith("data: ")) continue;
        const data = part.slice(6).trim();
        if (data === "[DONE]") return api.getDoc(docId);
        if (data.startsWith("[ERROR]")) throw new Error(data.slice(7).trim() || "stream error");
        try { onToken(JSON.parse(data) as string); } catch { /* skip malformed */ }
      }
    }
    throw new Error("stream closed without [DONE]");
  },

  // Feynman mode: teach a concept, get a supportive gap report (streamed).
  createFeynmanStream: (topic: string, explanation: string, onToken: (t: string) => void) =>
    streamSSE("/feynman", { topic, explanation, lang }, onToken),

  feynmanRoundStream: async (
    docId: string,
    explanation: string,
    onToken: (t: string) => void
  ): Promise<TutorDoc> => {
    await streamSSE(`/documents/${docId}/feynman`, { explanation, lang }, onToken);
    return api.getDoc(docId);
  },

  action: (docId: string, threadId: string, type: string) =>
    req<TutorDoc>(`/documents/${docId}/threads/${threadId}/actions`, {
      method: "POST",
      body: JSON.stringify({ type, lang }),
    }),
};
