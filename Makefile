# Tutor — dev orchestration.
# `make dev` brings up the whole stack: local LLM (Ollama + Gemma), Go API, web.

OLLAMA_MODEL ?= gemma2:12b
OLLAMA_HOST  ?= 127.0.0.1:11434
LLM_URL      ?= http://$(OLLAMA_HOST)/v1
PORT         ?= 8787

export TUTOR_LOCAL_LLM_URL   := $(LLM_URL)
export TUTOR_LOCAL_LLM_MODEL := $(OLLAMA_MODEL)
export PORT                  := $(PORT)

.PHONY: dev llm llm-up install server web build clean stop help

help: ## Show targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

## ---- Everything --------------------------------------------------------------

dev: llm-up install ## Bring up LLM + API + web (Ctrl-C stops all)
	@echo "→ tutor: API on :$(PORT), web on :5173, model=$(OLLAMA_MODEL)"
	@trap 'kill 0' INT TERM EXIT; \
		( cd server && go run . ) & \
		( cd web && npm run dev ) & \
		wait

## ---- Local LLM (Ollama) ------------------------------------------------------

llm: ## Pull the Gemma model
	@command -v ollama >/dev/null 2>&1 || { \
		echo "ollama not found — install from https://ollama.com (or set ANTHROPIC_API_KEY to use Claude)"; exit 0; }
	@echo "→ pulling $(OLLAMA_MODEL) (first run downloads several GB)"
	@OLLAMA_HOST=$(OLLAMA_HOST) ollama pull $(OLLAMA_MODEL)

llm-up: ## Ensure the Ollama server is running and the model is present
	@command -v ollama >/dev/null 2>&1 || { \
		echo "⚠ ollama not found — the API will fall back to its built-in mock LLM."; \
		echo "  Install from https://ollama.com, or set ANTHROPIC_API_KEY for Claude."; exit 0; }
	@curl -fsS http://$(OLLAMA_HOST)/api/version >/dev/null 2>&1 || { \
		echo "→ starting ollama serve"; \
		OLLAMA_HOST=$(OLLAMA_HOST) nohup ollama serve >/tmp/tutor-ollama.log 2>&1 & \
		sleep 2; }
	@OLLAMA_HOST=$(OLLAMA_HOST) ollama list 2>/dev/null | grep -q "$(OLLAMA_MODEL)" || $(MAKE) llm

## ---- Individual processes ----------------------------------------------------

install: ## Install web dependencies
	@cd web && [ -d node_modules ] || npm install

server: ## Run the Go API only
	@cd server && go run .

web: install ## Run the web dev server only
	@cd web && npm run dev

build: ## Build server binary + web bundle
	@cd server && go build -o ../bin/tutor-server .
	@cd web && npm install && npm run build

## ---- Housekeeping ------------------------------------------------------------

stop: ## Stop a background ollama started by llm-up
	@pkill -f "ollama serve" 2>/dev/null || true

clean: ## Remove build output and local data
	@rm -rf bin server/data web/dist
