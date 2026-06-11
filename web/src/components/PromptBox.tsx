import { useState } from "react";

export function PromptBox({
  onSubmit,
  busy,
  placeholder,
}: {
  onSubmit: (q: string) => void;
  busy: boolean;
  placeholder: string;
}) {
  const [value, setValue] = useState("");

  function submit() {
    const q = value.trim();
    if (!q || busy) return;
    onSubmit(q);
    setValue("");
  }

  return (
    <div className="prompt">
      <input
        value={value}
        placeholder={placeholder}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={(e) => e.key === "Enter" && submit()}
        enterKeyHint="go"
        autoFocus
      />
      <button className="btn btn-primary" onClick={submit} disabled={busy || !value.trim()}>
        {busy ? <span className="spinner" /> : "Ask"}
      </button>
    </div>
  );
}
