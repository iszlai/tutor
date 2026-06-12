import { useState } from "react";

// ExplainBox is the Explain-mode composer: you paste a document (a feature
// file, spec, one-pager, notes) and Tutor turns it into a one-page explanation.
export function ExplainBox({
  submitLabel,
  busy,
  onSubmit,
  placeholder,
}: {
  submitLabel: string;
  busy: boolean;
  onSubmit: (source: string) => void;
  placeholder: string;
}) {
  const [text, setText] = useState("");

  const canSubmit = !!text.trim() && !busy;

  function submit() {
    if (!canSubmit) return;
    onSubmit(text.trim());
    setText("");
  }

  return (
    <div className="teach">
      <textarea
        className="teach-text"
        value={text}
        placeholder={placeholder}
        onChange={(e) => setText(e.target.value)}
      />
      <button className="btn btn-primary teach-submit" onClick={submit} disabled={!canSubmit}>
        {busy ? <span className="spinner" /> : submitLabel}
      </button>
    </div>
  );
}
