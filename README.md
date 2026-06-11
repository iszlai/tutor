# Tutor — Interactive AI Learning Documents

> Mobile-first web app where an LLM-generated explanation becomes a *living
> document* you can annotate, discuss, and evolve — GitHub PR-review comments
> fused with Google Docs, but the AI is a participant in every thread.

This is the working scaffold. The full product spec lives in
[`docs/PLAN.md`](docs/PLAN.md).

## What works today (MVP slice)

- Ask a question → streamed one-page Markdown explanation (KaTeX math, code).
- Select any span of the doc → leave a comment → the AI replies in an anchored
  thread.
- Threads persist; anchors survive re-render via a 3-locator strategy
  (block id + char offset + quote/context fallback).
- Everything is persisted as **Markdown + canonical JSON** on disk, so a
  document is portable and reconstructable.

Runs out of the box with a **mock LLM** (no API key needed). Set
`ANTHROPIC_API_KEY` to use real Claude (`claude-opus-4-8`).

## Layout

```
docs/PLAN.md          full product + architecture plan
server/               Go API (LLM proxy + Markdown/JSON file store)
web/                  Vite + React + TypeScript PWA (mobile-first)
```

## Run

Two processes — the Go API and the Vite dev server (which proxies `/api`).

```bash
# 1. API  (terminal A)  — Go 1.22+
cd server
cp ../.env.example .env        # optional: add ANTHROPIC_API_KEY
go run .                       # listens on :8787

# 2. Web  (terminal B)  — Node 20+
cd web
npm install
npm run dev                    # http://localhost:5173  (proxies /api → :8787)
```

Open http://localhost:5173 on your phone (same wifi) or desktop.

## Stack & decisions

See [`docs/PLAN.md`](docs/PLAN.md) §2 for the rationale. Short version: the hard
problem is **stable text anchoring**, handled client-side in the DOM. The Go
backend is a thin LLM proxy + file store; LLM access sits behind a `Provider`
interface (`server/llm.go`) — Claude today, swappable later.
