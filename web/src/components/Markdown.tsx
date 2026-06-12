import ReactMarkdown from "react-markdown";
import type { Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import rehypeKatex from "rehype-katex";
import rehypeHighlight from "rehype-highlight";
import { Mermaid } from "./Mermaid";

const remarkPlugins = [remarkGfm, remarkMath];
// ignoreMissing: the `mermaid` language isn't registered with highlight.js; we
// render it ourselves, so don't let rehype-highlight throw on it.
const rehypePlugins = [rehypeKatex, [rehypeHighlight, { ignoreMissing: true }]];

// Collect the raw source text from a hast node (rehype-highlight may have split
// it into highlight spans, so read the tree rather than the rendered children).
function nodeText(node: any): string {
  if (!node) return "";
  if (node.type === "text") return node.value ?? "";
  return (node.children ?? []).map(nodeText).join("");
}

function hasMermaidClass(cls: unknown): boolean {
  return Array.isArray(cls) ? cls.includes("language-mermaid") : cls === "language-mermaid";
}

const components: Components = {
  code({ node, className, children, ...rest }) {
    if (hasMermaidClass(className)) {
      return <Mermaid code={nodeText(node).replace(/\n$/, "")} />;
    }
    return (
      <code className={className} {...rest}>
        {children}
      </code>
    );
  },
  // Drop the <pre> chrome around a mermaid block so the diagram stands alone.
  pre({ node, children, ...rest }) {
    const code = (node as any)?.children?.[0];
    if (code?.tagName === "code" && hasMermaidClass(code.properties?.className)) {
      return <>{children}</>;
    }
    return <pre {...rest}>{children}</pre>;
  },
};

export function Markdown({ children }: { children: string }) {
  return (
    <ReactMarkdown
      remarkPlugins={remarkPlugins}
      rehypePlugins={rehypePlugins as any}
      components={components}
    >
      {children}
    </ReactMarkdown>
  );
}
