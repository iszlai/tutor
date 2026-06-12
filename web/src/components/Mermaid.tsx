import { useEffect, useRef, useState } from "react";
import mermaid from "mermaid";

// Diagrams come from LLM output, so render with securityLevel "strict" — no
// embedded HTML or click handlers are honored.
mermaid.initialize({ startOnLoad: false, securityLevel: "strict", theme: "neutral" });

let counter = 0;

// Mermaid renders a ```mermaid fenced block into an SVG diagram. On a syntax
// error it falls back to showing the raw source so nothing is silently lost.
export function Mermaid({ code }: { code: string }) {
  const ref = useRef<HTMLDivElement>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    setError("");
    const id = `mermaid-${counter++}`;
    mermaid
      .render(id, code)
      .then(({ svg }) => {
        if (!cancelled && ref.current) ref.current.innerHTML = svg;
      })
      .catch((e) => {
        if (!cancelled) setError(String(e?.message ?? e));
      });
    return () => {
      cancelled = true;
    };
  }, [code]);

  if (error) {
    return (
      <pre className="mermaid-error">
        <code>{code}</code>
      </pre>
    );
  }
  return <div className="mermaid-diagram" ref={ref} />;
}
