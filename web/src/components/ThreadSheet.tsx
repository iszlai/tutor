import { useState } from "react";
import type { Thread } from "../types";
import { Markdown } from "./Markdown";

const ACTIONS: { type: string; label: string }[] = [
  { type: "createLinkedPage", label: "↳ Linked page" },
  { type: "rewrite", label: "✎ Rewrite" },
  { type: "insertSummary", label: "≡ Insert summary" },
  { type: "generateVisual", label: "◈ Visual" },
  { type: "generateExercise", label: "✓ Exercise" },
];

export function ThreadSheet({
  thread,
  quote,
  busy,
  onClose,
  onSend,
  onAction,
}: {
  thread?: Thread;
  quote: string;
  busy: boolean;
  onClose: () => void;
  onSend: (text: string) => void;
  onAction: (type: string) => void;
}) {
  const [text, setText] = useState("");

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
            On <mark>{quote}</mark>
          </div>
        </div>

        <div className="messages">
          {thread?.messages.map((m) => (
            <div key={m.id} className={`msg ${m.role}`}>
              <div className="who">{m.role === "user" ? "You" : "Tutor"}</div>
              <div className="bubble">
                <Markdown>{m.text}</Markdown>
              </div>
            </div>
          ))}
          {!thread && (
            <p className="hint">Ask anything about the highlighted text.</p>
          )}
          {busy && (
            <div className="msg assistant">
              <div className="who">Tutor</div>
              <div className="bubble">…</div>
            </div>
          )}
        </div>

        {thread && (
          <div className="actions-row">
            {ACTIONS.map((a) => (
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
            placeholder={thread ? "Reply…" : "What do you want to know?"}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && send()}
            enterKeyHint="send"
            autoFocus
          />
          <button className="btn btn-primary" onClick={send} disabled={busy || !text.trim()}>
            {busy ? <span className="spinner" /> : "Send"}
          </button>
        </div>
      </div>
    </>
  );
}
