import { useState } from "react";

// TeachBox is the Feynman-mode composer: you explain a concept in your own
// words. With showTopic it also asks what concept you're teaching (used to start
// a session); without it, it's the "explain again" box for a refine pass.
export function TeachBox({
  showTopic,
  submitLabel,
  busy,
  onSubmit,
}: {
  showTopic: boolean;
  submitLabel: string;
  busy: boolean;
  onSubmit: (topic: string, explanation: string) => void;
}) {
  const [topic, setTopic] = useState("");
  const [text, setText] = useState("");

  const canSubmit = !!text.trim() && (!showTopic || !!topic.trim()) && !busy;

  function submit() {
    if (!canSubmit) return;
    onSubmit(topic.trim(), text.trim());
    setText("");
    if (showTopic) setTopic("");
  }

  return (
    <div className="teach">
      {showTopic && (
        <input
          className="teach-topic"
          value={topic}
          placeholder="What concept are you teaching? e.g. recursion"
          onChange={(e) => setTopic(e.target.value)}
        />
      )}
      <textarea
        className="teach-text"
        value={text}
        placeholder="Explain it in your own words, as if to a curious beginner…"
        onChange={(e) => setText(e.target.value)}
      />
      <button className="btn btn-primary teach-submit" onClick={submit} disabled={!canSubmit}>
        {busy ? <span className="spinner" /> : submitLabel}
      </button>
    </div>
  );
}
