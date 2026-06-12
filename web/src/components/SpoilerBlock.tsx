import { useState } from "react";

export function SpoilerBlock({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="spoiler-block">
      <button className="spoiler-toggle" onClick={() => setOpen((o) => !o)}>
        {open ? "Hide solution" : "Show solution"}
      </button>
      {open && <div className="spoiler-content">{children}</div>}
    </div>
  );
}
