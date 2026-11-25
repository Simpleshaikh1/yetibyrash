import type { ReactNode } from "react";

// A small Markdown reader for the shop's pages (About, FAQ, policies). It understands
// what staff need and nothing more: ## and ### headings, paragraphs, - and 1. lists,
// **bold**, and [links](/shop/bags). It builds React elements, never raw HTML, so page
// text can't inject anything into the shop.

type Inline = string | { bold: string } | { text: string; href: string };
type Block =
  | { kind: "h2" | "h3" | "p"; parts: Inline[] }
  | { kind: "ul" | "ol"; items: Inline[][] };

/** Links may go to the shop, a secure site, email or phone; anything else shows as plain text. */
function safeHref(href: string): string | null {
  if (href.startsWith("/") && !href.startsWith("//")) return href;
  if (/^(https:\/\/|mailto:|tel:)/i.test(href)) return href;
  return null;
}

function inline(text: string): Inline[] {
  const out: Inline[] = [];
  const re = /\*\*(.+?)\*\*|\[([^\]]+)\]\(([^)\s]+)\)/g;
  let last = 0;
  for (let m = re.exec(text); m; m = re.exec(text)) {
    if (m.index > last) out.push(text.slice(last, m.index));
    if (m[1] !== undefined) {
      out.push({ bold: m[1] });
    } else {
      const href = safeHref(m[3]);
      out.push(href ? { text: m[2], href } : m[2]);
    }
    last = re.lastIndex;
  }
  if (last < text.length) out.push(text.slice(last));
  return out;
}

export function parseMarkdown(src: string): Block[] {
  const blocks: Block[] = [];
  let para: string[] = [];
  let list: { kind: "ul" | "ol"; items: Inline[][] } | null = null;
  const flush = () => {
    if (para.length) blocks.push({ kind: "p", parts: inline(para.join(" ")) });
    para = [];
    if (list) blocks.push(list);
    list = null;
  };
  for (const raw of src.replace(/\r\n/g, "\n").split("\n")) {
    const line = raw.trim();
    const bullet = /^[-*]\s+(.*)$/.exec(line);
    const numbered = /^\d+[.)]\s+(.*)$/.exec(line);
    if (!line) {
      flush();
    } else if (line.startsWith("### ")) {
      flush();
      blocks.push({ kind: "h3", parts: inline(line.slice(4)) });
    } else if (line.startsWith("## ") || line.startsWith("# ")) {
      flush();
      blocks.push({ kind: "h2", parts: inline(line.replace(/^#+\s+/, "")) });
    } else if (bullet || numbered) {
      const kind = bullet ? "ul" : "ol";
      if (para.length || (list && list.kind !== kind)) flush();
      list ??= { kind, items: [] };
      list.items.push(inline((bullet ?? numbered)![1]));
    } else {
      if (list) flush();
      para.push(line);
    }
  }
  flush();
  return blocks;
}

function renderInline(parts: Inline[]): ReactNode[] {
  return parts.map((p, i) => {
    if (typeof p === "string") return p;
    if ("bold" in p) return <strong key={i}>{p.bold}</strong>;
    const external = !p.href.startsWith("/");
    return (
      <a key={i} href={p.href} {...(external ? { target: "_blank", rel: "noreferrer" } : {})}>
        {p.text}
      </a>
    );
  });
}

/** Renders page text written in light Markdown. */
export function Markdown({ source, className }: { source: string; className?: string }) {
  return (
    <div className={className}>
      {parseMarkdown(source).map((b, i) => {
        switch (b.kind) {
          case "h2":
            return <h2 key={i}>{renderInline(b.parts)}</h2>;
          case "h3":
            return <h3 key={i}>{renderInline(b.parts)}</h3>;
          case "p":
            return <p key={i}>{renderInline(b.parts)}</p>;
          case "ul":
            return (
              <ul key={i}>
                {b.items.map((it, j) => (
                  <li key={j}>{renderInline(it)}</li>
                ))}
              </ul>
            );
          case "ol":
            return (
              <ol key={i}>
                {b.items.map((it, j) => (
                  <li key={j}>{renderInline(it)}</li>
                ))}
              </ol>
            );
        }
      })}
    </div>
  );
}
