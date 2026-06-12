import { useCallback, useEffect, useState } from "react";
import type { Anchor, DocSummary, SearchHit, TutorDoc } from "./types";
import { api } from "./api";
import { type Lang, getStoredLang, storeLang, strings } from "./i18n";
import { buildAnchor } from "./anchor";
import { PromptBox } from "./components/PromptBox";
import { TeachBox } from "./components/TeachBox";
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
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchHit[]>([]);
  const [importOpen, setImportOpen] = useState(false);
  const [importMd, setImportMd] = useState("");
  const [streamingReply, setStreamingReply] = useState("");
  const [streamingDoc, setStreamingDoc] = useState<{ question: string; text: string } | null>(null);
  const [mode, setMode] = useState<"learn" | "teach">("learn");
  const [streamingRound, setStreamingRound] = useState<string | null>(null);
  const [lang, setLang] = useState<Lang>(getStoredLang);
  const t = strings(lang);

  // Keep the API client's language in sync; persist the choice across reloads.
  useEffect(() => {
    api.setLang(lang);
    storeLang(lang);
  }, [lang]);

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

  // Reset the search box whenever the dropdown closes (any path).
  useEffect(() => {
    if (!historyOpen) setQuery("");
  }, [historyOpen]);

  // Debounced library search; empty query falls back to the recents list.
  useEffect(() => {
    const q = query.trim();
    if (!q) {
      setResults([]);
      return;
    }
    const id = setTimeout(() => {
      api.search(q).then(setResults).catch(() => setResults([]));
    }, 200);
    return () => clearTimeout(id);
  }, [query]);

  async function deleteDoc(id: string) {
    if (!confirm(t.deleteConfirm)) return;
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

  // Feynman mode: start a session by teaching a concept.
  async function teach(topic: string, explanation: string) {
    setError("");
    setBusy(true);
    setStreamingDoc({ question: "Feynman: " + topic, text: "" });
    try {
      const docId = await api.createFeynmanStream(topic, explanation, (token) => {
        setStreamingDoc((prev) => (prev ? { ...prev, text: prev.text + token } : null));
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

  // Feynman mode: explain again to get a fresh gap report appended.
  async function refine(_topic: string, explanation: string) {
    if (!doc) return;
    setError("");
    setBusy(true);
    setStreamingRound("");
    try {
      const updated = await api.feynmanRoundStream(doc.id, explanation, (token) => {
        setStreamingRound((prev) => (prev ?? "") + token);
      });
      setStreamingRound(null);
      setDoc(updated);
      location.hash = updated.id;
    } catch (e) {
      setError(String(e));
      setStreamingRound(null);
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
          <div className="lang-toggle" role="group" aria-label="Language">
            <button
              className={`lang-btn${lang === "en" ? " lang-btn--active" : ""}`}
              onClick={() => setLang("en")}
            >
              EN
            </button>
            <button
              className={`lang-btn${lang === "hu" ? " lang-btn--active" : ""}`}
              onClick={() => setLang("hu")}
            >
              HU
            </button>
          </div>
          {doc && (
            <>
              <button className="btn btn-ghost btn-sm" onClick={newSession}>
                {t.new}
              </button>
              <button
                className="btn btn-ghost btn-sm btn-danger"
                onClick={() => deleteDoc(doc.id)}
                title={t.deleteTitle}
              >
                {t.delete}
              </button>
            </>
          )}
          <div className="history-wrap">
            <button className="btn btn-ghost btn-sm" onClick={toggleHistory}>
              {t.history} {historyOpen ? "▲" : "▼"}
            </button>
            {historyOpen && (
              <div className="history-dropdown">
                <input
                  className="history-search"
                  placeholder={t.searchLibrary}
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  autoFocus
                />
                {query.trim() ? (
                  results.length === 0 ? (
                    <p className="history-empty">{t.noMatches}</p>
                  ) : (
                    results.map((r) => (
                      <a
                        key={r.id}
                        className="search-hit"
                        onClick={() => loadDoc(r.id)}
                      >
                        <span className="search-hit-title">{r.title}</span>
                        {r.snippet && (
                          <span className="search-hit-snippet">{r.snippet}</span>
                        )}
                      </a>
                    ))
                  )
                ) : recents.length === 0 ? (
                  <p className="history-empty">{t.noSessions}</p>
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

      <div className="mode-toggle" role="tablist" aria-label="Mode">
        <button
          className={`mode-tab${mode === "learn" ? " mode-tab--active" : ""}`}
          onClick={() => setMode("learn")}
        >
          {t.learn}
        </button>
        <button
          className={`mode-tab${mode === "teach" ? " mode-tab--active" : ""}`}
          onClick={() => setMode("teach")}
          title={t.teachTitle}
        >
          {t.teach}
        </button>
      </div>

      {mode === "learn" ? (
        <PromptBox
          onSubmit={ask}
          busy={busy && !doc}
          placeholder={t.askPlaceholder}
          submitLabel={t.ask}
        />
      ) : (
        <TeachBox
          showTopic
          submitLabel={t.getFeedback}
          busy={busy && !doc}
          onSubmit={teach}
          topicPlaceholder={t.topicPlaceholder}
          explainPlaceholder={t.explainPlaceholder}
        />
      )}

      {mode === "teach" && <p className="hint">{t.feynmanIntro}</p>}

      {mode === "learn" && (
        <>
          <div className="import-toggle">
            <button
              className="import-toggle-btn"
              onClick={() => setImportOpen((o) => !o)}
            >
              {importOpen ? t.pasteCancel : t.pasteOpen}
            </button>
          </div>

          {importOpen && (
            <div className="import-form">
              <textarea
                className="import-textarea"
                placeholder={t.pastePlaceholder}
                value={importMd}
                onChange={(e) => setImportMd(e.target.value)}
                autoFocus
              />
              <button
                className="btn btn-primary"
                onClick={importDoc}
                disabled={busy || !importMd.trim()}
              >
                {busy ? <span className="spinner" /> : t.import}
              </button>
            </div>
          )}

          {!doc && <p className="hint">{t.learnIntro}</p>}
        </>
      )}

      {error && <div className="error">{error}</div>}

      {!doc && recents.length > 0 && (
        <div className="recents">
          <h3>{t.recent}</h3>
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

      {!streamingDoc && doc?.mode === "feynman" && (
        <div className="feynman-legend">
          <span className="fey-chip fey-chip--gap">{t.feyGap}</span>
          <span className="fey-chip fey-chip--jargon">{t.feyJargon}</span>
          <span className="fey-chip fey-chip--shaky">{t.feyShaky}</span>
          <span className="feynman-legend-note">{t.feyLegendNote}</span>
        </div>
      )}

      {!streamingDoc && doc && (
        <DocumentView
          doc={doc}
          onOpenThread={(threadId) => setSheet({ mode: "thread", threadId })}
          onNavigate={loadDoc}
        />
      )}

      {!streamingDoc && doc?.mode === "feynman" && (
        <div className="feynman-refine">
          {streamingRound !== null && (
            <div className="block block--feynman-report">
              <Markdown>{streamingRound || "…"}</Markdown>
            </div>
          )}
          <h3 className="feynman-refine-title">{t.explainAgainTitle}</h3>
          <p className="hint">{t.explainAgainHint}</p>
          <TeachBox
            showTopic={false}
            submitLabel={t.explainAgain}
            busy={busy}
            onSubmit={refine}
            topicPlaceholder={t.topicPlaceholder}
            explainPlaceholder={t.explainPlaceholder}
          />
        </div>
      )}

      {toolbar && !sheet && (
        <SelectionToolbar x={toolbar.x} y={toolbar.y} flip={toolbar.flip} onComment={startComment} t={t} />
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
          t={t}
        />
      )}
    </div>
  );
}
