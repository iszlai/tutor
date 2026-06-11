# Tutor — dev orchestration.
# `make dev` brings up the whole stack: local LLM (Ollama + Gemma), Go API, web.

OLLAMA_MODEL ?= gemma3:12b
OLLAMA_HOST  ?= 127.0.0.1:11434
LLM_URL      ?= http://$(OLLAMA_HOST)/v1
PORT         ?= 8787

export TUTOR_LOCAL_LLM_URL   := $(LLM_URL)
export TUTOR_LOCAL_LLM_MODEL := $(OLLAMA_MODEL)
export PORT                  := $(PORT)

.PHONY: dev llm llm-up install server web build clean stop help electron-dev electron-dist

help: ## Show targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

## ---- Everything --------------------------------------------------------------

dev: llm-up install ## Bring up LLM + API + web in browser (Ctrl-C stops all)
	@echo "→ tutor: starting Go server (compiling…)"
	@trap 'kill 0' INT TERM EXIT; \
		( cd server && go run . ) & \
		until curl -fsS http://localhost:$(PORT)/api/health >/dev/null 2>&1; do sleep 0.3; done; \
		echo "→ tutor: API ready on :$(PORT), web on :5173, model=$(OLLAMA_MODEL)"; \
		( cd web && npm run dev ) & \
		wait

dev-m1: ## Like dev, but uses gemma3:4b (~3.3 GB) for 16 GB M1 Macs
	OLLAMA_MODEL=gemma3:4b $(MAKE) dev

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

## ---- Electron ---------------------------------------------------------------

electron-dev: ## Hot-reload Electron dev: starts Vite + Go (via go run) + Ollama
	@cd web && [ -d node_modules ] || npm install
	@[ -d node_modules ] || npm install
	@echo "→ tutor: Electron dev (Vite :5173, API :$(PORT), model=$(OLLAMA_MODEL))"
	@trap 'kill 0' INT TERM EXIT; \
		( cd web && npm run dev ) & \
		ELECTRON_DEV=1 TUTOR_LOCAL_LLM_MODEL=$(OLLAMA_MODEL) \
			./node_modules/.bin/electron . ; \
		wait

electron-dist: ## Build Go binary + web bundle + package .dmg → dist-electron/
	@echo "→ building Go server binary"
	@mkdir -p bin
	@cd server && go build -o ../bin/tutor-server .
	@echo "→ building web bundle"
	@cd web && npm install && VITE_API_BASE=http://localhost:$(PORT) npm run build
	@echo "→ packaging Electron app"
	@[ -d node_modules ] || npm install
	@npm run dist

## ---- Housekeeping ------------------------------------------------------------

stop: ## Stop a background ollama started by llm-up
	@pkill -f "ollama serve" 2>/dev/null || true

clean: ## Remove build output and local data
	@rm -rf bin server/data web/dist dist-electron
