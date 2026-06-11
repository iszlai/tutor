import { useCallback, useEffect, useState } from "react";
import type { Anchor, DocSummary, TutorDoc } from "./types";
import { api } from "./api";
import { buildAnchor } from "./anchor";
import { PromptBox } from "./components/PromptBox";
import { DocumentView } from "./components/DocumentView";
import { SelectionToolbar } from "./components/SelectionToolbar";
import { ThreadSheet } from "./components/ThreadSheet";

type Sheet =
  | { mode: "new"; anchor: Anchor }
  | { mode: "thread"; threadId: string }
  | null;

type Toolbar = { anchor: Anchor; x: number; y: number } | null;

export default function App() {
  const [doc, setDoc] = useState<TutorDoc | null>(null);
  const [recents, setRecents] = useState<DocSummary[]>([]);
  const [provider, setProvider] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [toolbar, setToolbar] = useState<Toolbar>(null);
  const [sheet, setSheet] = useState<Sheet>(null);

  // --- initial load -------------------------------------------------------
  useEffect(() => {
    api.health().then((h) => setProvider(`${h.provider} · ${h.model}`)).catch(() => {});
    api.listDocs().then(setRecents).catch(() => {});
    const id = location.hash.slice(1);
    if (id) loadDoc(id);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const loadDoc = useCallback((id: string) => {
    setSheet(null);
    api
      .getDoc(id)
      .then((d) => {
        setDoc(d);
        location.hash = d.id;
      })
      .catch((e) => setError(String(e)));
  }, []);

  // --- selection → floating toolbar --------------------------------------
  useEffect(() => {
    function onSelectionChange() {
      if (sheet) return; // don't fight with the open sheet
      const sel = window.getSelection();
      const anchor = sel ? buildAnchor(sel) : null;
      if (!anchor || !sel) {
        setToolbar(null);
        return;
      }
      const rect = sel.getRangeAt(0).getBoundingClientRect();
      setToolbar({ anchor, x: rect.left + rect.width / 2, y: rect.top - 8 });
    }
    document.addEventListener("selectionchange", onSelectionChange);
    return () => document.removeEventListener("selectionchange", onSelectionChange);
  }, [sheet]);

  // --- actions ------------------------------------------------------------
  async function ask(question: string) {
    setError("");
    setBusy(true);
    try {
      const d = await api.createDoc(question);
      setDoc(d);
      location.hash = d.id;
      api.listDocs().then(setRecents).catch(() => {});
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  }

  function startComment() {
    if (!toolbar) return;
    setSheet({ mode: "new", anchor: toolbar.anchor });
    setToolbar(null);
    window.getSelection()?.removeAllRanges();
  }

  async function sheetSend(text: string) {
    if (!doc || !sheet) return;
    setBusy(true);
    setError("");
    try {
      if (sheet.mode === "new") {
        const d = await api.createThread(doc.id, sheet.anchor, text);
        setDoc(d);
        const newest = d.threads[d.threads.length - 1];
        setSheet({ mode: "thread", threadId: newest.threadId });
      } else {
        setDoc(await api.reply(doc.id, sheet.threadId, text));
      }
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  }

  async function runAction(type: string) {
    if (!doc || sheet?.mode !== "thread") return;
    setBusy(true);
    setError("");
    try {
      setDoc(await api.action(doc.id, sheet.threadId, type));
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  }

  const activeThread =
    sheet?.mode === "thread" ? doc?.threads.find((t) => t.threadId === sheet.threadId) : undefined;
  const sheetQuote =
    sheet?.mode === "new" ? sheet.anchor.exactQuote : activeThread?.anchor.exactQuote ?? "";

  return (
    <div className="app">
      <div className="topbar">
        <span className="brand">
          <span className="brand-dot" />
          Tutor
        </span>
        {provider && <span className="provider-pill">{provider}</span>}
      </div>

      <PromptBox
        onSubmit={ask}
        busy={busy && !doc}
        placeholder="Ask anything — e.g. how do you calculate acceleration?"
      />
      {!doc && (
        <p className="hint">
          Get a one-page explanation, then select any word, formula, or sentence to ask
          a follow-up. The AI replies in a thread you can turn into linked pages,
          rewrites, summaries, visuals, or exercises.
        </p>
      )}

      {error && <div className="error">{error}</div>}

      {!doc && recents.length > 0 && (
        <div className="recents">
          <h3>Recent</h3>
          {recents.map((r) => (
            <a key={r.id} onClick={() => loadDoc(r.id)} href={`#${r.id}`}>
              {r.title}
            </a>
          ))}
        </div>
      )}

      {doc && (
        <DocumentView
          doc={doc}
          onOpenThread={(threadId) => setSheet({ mode: "thread", threadId })}
          onNavigate={loadDoc}
        />
      )}

      {toolbar && !sheet && (
        <SelectionToolbar x={toolbar.x} y={toolbar.y} onComment={startComment} />
      )}

      {sheet && (
        <ThreadSheet
          thread={activeThread}
          quote={sheetQuote}
          busy={busy}
          onClose={() => setSheet(null)}
          onSend={sheetSend}
          onAction={runAction}
        />
      )}
    </div>
  );
}
