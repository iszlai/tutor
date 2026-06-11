import { useEffect, useRef } from "react";
import type { DocLink, Thread, TutorDoc } from "../types";
import { paintHighlights } from "../anchor";
import { Markdown } from "./Markdown";

export function DocumentView({
  doc,
  onOpenThread,
  onNavigate,
}: {
  doc: TutorDoc;
  onOpenThread: (threadId: string) => void;
  onNavigate: (docId: string) => void;
}) {
  const ref = useRef<HTMLDivElement>(null);

  // Repaint anchored-comment highlights whenever the doc changes.
  useEffect(() => {
    if (!ref.current) return;
    paintHighlights(
      ref.current,
      doc.threads.map((t) => ({ threadId: t.threadId, anchor: t.anchor }))
    );
  }, [doc]);

  const threadsByBlock = new Map<string, Thread[]>();
  for (const t of doc.threads) {
    const list = threadsByBlock.get(t.anchor.startBlockId) ?? [];
    list.push(t);
    threadsByBlock.set(t.anchor.startBlockId, list);
  }

  const children = doc.links.filter((l) => l.type === "child");
  const related = doc.links.filter((l) => l.type === "related");

  return (
    <div>
      <h1 className="doc-title">{doc.title}</h1>

      {(children.length > 0 || related.length > 0) && (
        <div className="breadcrumb">
          {related.map((l) => (
            <LinkChip key={l.targetDocId} link={l} prefix="↑" onNavigate={onNavigate} />
          ))}
          {children.map((l) => (
            <LinkChip key={l.targetDocId} link={l} prefix="↳" onNavigate={onNavigate} />
          ))}
        </div>
      )}

      <div ref={ref}>
        {doc.blocks.map((b) => {
          const threads = threadsByBlock.get(b.blockId) ?? [];
          return (
            <div key={b.blockId} className="block" data-block-id={b.blockId}>
              <Markdown>{b.markdown}</Markdown>
              {threads.map((t) => (
                <span
                  key={t.threadId}
                  className="thread-marker"
                  onClick={() => onOpenThread(t.threadId)}
                  title="Open discussion"
                >
                  💬 {t.messages.filter((m) => m.role === "user").length}
                </span>
              ))}
            </div>
          );
        })}
      </div>
    </div>
  );
}

function LinkChip({
  link,
  prefix,
  onNavigate,
}: {
  link: DocLink;
  prefix: string;
  onNavigate: (id: string) => void;
}) {
  return (
    <span className="chip" onClick={() => onNavigate(link.targetDocId)}>
      {prefix} {link.label}
    </span>
  );
}
