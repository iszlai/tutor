import { useState } from "react";
import type { Thread } from "../types";
import type { Strings } from "../i18n";
import { Markdown } from "./Markdown";

export function ThreadSheet({
  thread,
  quote,
  busy,
  streamingReply,
  onClose,
  onSend,
  onAction,
  t,
}: {
  thread?: Thread;
  quote: string;
  busy: boolean;
  streamingReply?: string;
  onClose: () => void;
  onSend: (text: string) => void;
  onAction: (type: string) => void;
  t: Strings;
}) {
  const [text, setText] = useState("");

  const actions: { type: string; label: string }[] = [
    { type: "createLinkedPage", label: t.actLinkedPage },
    { type: "rewrite", label: t.actRewrite },
    { type: "insert", label: t.actInsert },
    { type: "insertSummary", label: t.actInsertSummary },
    { type: "generateVisual", label: t.actVisual },
    { type: "generateExercise", label: t.actExercise },
  ];

  function send() {
    const t = text.trim();
    if (!t || busy) return;
    onSend(t);
    setText("");
  }

  return (
    <>
      <div className="scrim" onClick={onClose} />
      <div className="sheet" role="dialog" aria-label="Discussion">
        <div className="sheet-grab" />
        <div className="sheet-head">
          <div className="sheet-quote">
            {t.on} <mark>{quote}</mark>
          </div>
        </div>

        <div className="messages">
          {thread?.messages.map((m) => (
            <div key={m.id} className={`msg ${m.role}`}>
              <div className="who">{m.role === "user" ? t.you : t.tutor}</div>
              <div className="bubble">
                <Markdown>{m.text}</Markdown>
              </div>
            </div>
          ))}
          {!thread && (
            <p className="hint">{t.askHighlighted}</p>
          )}
          {streamingReply && (
            <div className="msg assistant">
              <div className="who">{t.tutor}</div>
              <div className="bubble">
                <Markdown>{streamingReply}</Markdown>
              </div>
            </div>
          )}
          {busy && !streamingReply && (
            <div className="msg assistant">
              <div className="who">{t.tutor}</div>
              <div className="bubble">…</div>
            </div>
          )}
        </div>

        {thread && (
          <div className="actions-row">
            {actions.map((a) => (
              <button
                key={a.type}
                className="btn btn-ghost btn-sm"
                disabled={busy}
                onClick={() => onAction(a.type)}
              >
                {a.label}
              </button>
            ))}
          </div>
        )}

        <div className="composer">
          <input
            value={text}
            placeholder={thread ? t.replyPlaceholder : t.newThreadPlaceholder}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && send()}
            enterKeyHint="send"
            autoFocus
          />
          <button className="btn btn-primary" onClick={send} disabled={busy || !text.trim()}>
            {busy ? <span className="spinner" /> : t.send}
          </button>
        </div>
      </div>
    </>
  );
}
