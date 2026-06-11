import { useCallback, useEffect, useState } from "react";
import type { Anchor, DocSummary, TutorDoc } from "./types";
import { api } from "./api";
import { buildAnchor } from "./anchor";
import { PromptBox } from "./components/PromptBox";
import { DocumentView } from "./components/DocumentView";
import { Markdown } from "./components/Markdown";
import { SelectionToolbar } from "./components/SelectionToolbar";
import { ThreadSheet } from "./components/ThreadSheet";

type Sheet =
  | { mode: "new"; anchor: Anchor }
  | { mode: "thread"; threadId: string }
  | null;

type Toolbar = { anchor: Anchor; x: number; y: number; flip: boolean } | null;

export default function App() {
  const [doc, setDoc] = useState<TutorDoc | null>(null);
  const [recents, setRecents] = useState<DocSummary[]>([]);
  const [provider, setProvider] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [toolbar, setToolbar] = useState<Toolbar>(null);
  const [sheet, setSheet] = useState<Sheet>(null);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [importMd, setImportMd] = useState("");
  const [streamingReply, setStreamingReply] = useState("");
  const [streamingDoc, setStreamingDoc] = useState<{ question: string; text: string } | null>(null);

  // --- initial load -------------------------------------------------------
  useEffect(() => {
    api.health().then((h) => setProvider(`${h.provider} · ${h.model}`)).catch(() => {});
    api.listDocs().then(setRecents).catch(() => {});
    const id = location.hash.slice(1);
    if (id) loadDoc(id);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const loadDoc = useCallback((id: string) => {
    setSheet(null);
    setHistoryOpen(false);
    api
      .getDoc(id)
      .then((d) => {
        setDoc(d);
        location.hash = d.id;
      })
      .catch((e) => setError(String(e)));
  }, []);

  function newSession() {
    setDoc(null);
    setSheet(null);
    setHistoryOpen(false);
    location.hash = "";
    api.listDocs().then(setRecents).catch(() => {});
  }

  function toggleHistory() {
    if (!historyOpen) api.listDocs().then(setRecents).catch(() => {});
    setHistoryOpen((o) => !o);
  }

  async function deleteDoc(id: string) {
    if (!confirm("Delete this session? This cannot be undone.")) return;
    await api.deleteDoc(id);
    if (doc?.id === id) newSession();
    api.listDocs().then(setRecents).catch(() => {});
  }

  // --- selection → floating toolbar --------------------------------------
  useEffect(() => {
    const TOOLBAR_H = 44;
    const TOOLBAR_HALF_W = 70;
    let timer: ReturnType<typeof setTimeout>;

    function readSelection() {
      if (sheet) return;
      const sel = window.getSelection();
      const anchor = sel ? buildAnchor(sel) : null;
      if (!anchor || !sel) {
        setToolbar(null);
        return;
      }
      const rect = sel.getRangeAt(0).getBoundingClientRect();
      if (rect.width === 0 && rect.height === 0) return; // mid-drag on mobile

      const cx = Math.max(
        TOOLBAR_HALF_W,
        Math.min(window.innerWidth - TOOLBAR_HALF_W, rect.left + rect.width / 2)
      );
      const spaceAbove = rect.top;
      const flip = spaceAbove < TOOLBAR_H + 12;
      const y = flip ? rect.bottom + 8 : rect.top - 8;
      setToolbar({ anchor, x: cx, y, flip });
    }

    // Debounce selectionchange — fires many times while dragging handles on mobile.
    function onSelectionChange() {
      clearTimeout(timer);
      timer = setTimeout(readSelection, 120);
    }

    // pointerup / touchend: read immediately after the user lifts their finger.
    function onPointerUp() {
      clearTimeout(timer);
      timer = setTimeout(readSelection, 30);
    }

    document.addEventListener("selectionchange", onSelectionChange);
    document.addEventListener("pointerup", onPointerUp);
    document.addEventListener("touchend", onPointerUp);
    return () => {
      clearTimeout(timer);
      document.removeEventListener("selectionchange", onSelectionChange);
      document.removeEventListener("pointerup", onPointerUp);
      document.removeEventListener("touchend", onPointerUp);
    };
  }, [sheet]);

  // --- actions ------------------------------------------------------------
  async function importDoc() {
    const md = importMd.trim();
    if (!md || busy) return;
    setError("");
    setBusy(true);
    try {
      const d = await api.importDoc(md);
      setDoc(d);
      location.hash = d.id;
      setImportOpen(false);
      setImportMd("");
      api.listDocs().then(setRecents).catch(() => {});
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  }

  async function ask(question: string) {
    setError("");
    setBusy(true);
    setStreamingDoc({ question, text: "" });
    try {
      const docId = await api.createDocStream(question, (token) => {
        setStreamingDoc((prev) => prev ? { ...prev, text: prev.text + token } : null);
      });
      setStreamingDoc(null);
      await loadDoc(docId);
      api.listDocs().then(setRecents).catch(() => {});
    } catch (e) {
      setError(String(e));
      setStreamingDoc(null);
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
        setStreamingReply("");
        const d = await api.replyStream(doc.id, sheet.threadId, text, (token) => {
          setStreamingReply((prev) => prev + token);
        });
        setStreamingReply("");
        setDoc(d);
      }
    } catch (e) {
      setError(String(e));
      setStreamingReply("");
    } finally {
      setBusy(false);
    }
  }

  async function runAction(type: string) {
    if (!doc || sheet?.mode !== "thread") return;
    setBusy(true);
    setError("");
    try {
      const result = await api.action(doc.id, sheet.threadId, type);
      setDoc(result);
      location.hash = result.id;
      if (type === "createLinkedPage") {
        setSheet(null);
        api.listDocs().then(setRecents).catch(() => {});
      }
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
        <button className="brand brand-btn" onClick={newSession}>
          <span className="brand-dot" />
          Tutor
        </button>
        <div className="topbar-actions">
          {doc && (
            <>
              <button className="btn btn-ghost btn-sm" onClick={newSession}>
                + New
              </button>
              <button
                className="btn btn-ghost btn-sm btn-danger"
                onClick={() => deleteDoc(doc.id)}
                title="Delete this session"
              >
                Delete
              </button>
            </>
          )}
          <div className="history-wrap">
            <button className="btn btn-ghost btn-sm" onClick={toggleHistory}>
              History {historyOpen ? "▲" : "▼"}
            </button>
            {historyOpen && (
              <div className="history-dropdown">
                {recents.length === 0 ? (
                  <p className="history-empty">No sessions yet</p>
                ) : (
                  recents.map((r) => (
                    <div key={r.id} className="history-item-row">
                      <a className="history-item" onClick={() => loadDoc(r.id)}>
                        {r.title}
                      </a>
                      <button
                        className="history-delete"
                        title="Delete"
                        onClick={(e) => { e.stopPropagation(); deleteDoc(r.id); }}
                      >
                        ✕
                      </button>
                    </div>
                  ))
                )}
              </div>
            )}
          </div>
          {provider && <span className="provider-pill">{provider}</span>}
        </div>
      </div>
      {historyOpen && <div className="history-scrim" onClick={() => setHistoryOpen(false)} />}

      <PromptBox
        onSubmit={ask}
        busy={busy && !doc}
        placeholder="Ask anything — e.g. how do you calculate acceleration?"
      />

      <div className="import-toggle">
        <button
          className="import-toggle-btn"
          onClick={() => setImportOpen((o) => !o)}
        >
          {importOpen ? "▲ Cancel" : "↑ Paste a .md file instead"}
        </button>
      </div>

      {importOpen && (
        <div className="import-form">
          <textarea
            className="import-textarea"
            placeholder="Paste your markdown here…"
            value={importMd}
            onChange={(e) => setImportMd(e.target.value)}
            autoFocus
          />
          <button
            className="btn btn-primary"
            onClick={importDoc}
            disabled={busy || !importMd.trim()}
          >
            {busy ? <span className="spinner" /> : "Import"}
          </button>
        </div>
      )}

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

      {streamingDoc && (
        <div>
          <h1 className="doc-title">{streamingDoc.question}</h1>
          <div className="block">
            <Markdown>{streamingDoc.text || "…"}</Markdown>
          </div>
        </div>
      )}

      {!streamingDoc && doc && (
        <DocumentView
          doc={doc}
          onOpenThread={(threadId) => setSheet({ mode: "thread", threadId })}
          onNavigate={loadDoc}
        />
      )}

      {toolbar && !sheet && (
        <SelectionToolbar x={toolbar.x} y={toolbar.y} flip={toolbar.flip} onComment={startComment} />
      )}

      {sheet && (
        <ThreadSheet
          thread={activeThread}
          quote={sheetQuote}
          busy={busy}
          streamingReply={streamingReply}
          onClose={() => setSheet(null)}
          onSend={sheetSend}
          onAction={runAction}
        />
      )}
    </div>
  );
}
