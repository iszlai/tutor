package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func decode(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return false
	}
	return true
}

func statusFor(err error) int {
	if errors.Is(err, errNotFound) {
		return http.StatusNotFound
	}
	if errors.Is(err, errBadAction) {
		return http.StatusBadRequest
	}
	return http.StatusBadGateway
}

// ---- Server-sent events ----------------------------------------------------

func sseHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
}

// sseToken returns an onToken callback that emits each chunk as a JSON-encoded
// SSE data frame, matching what the frontend stream readers expect.
func sseToken(w http.ResponseWriter, f http.Flusher) func(string) {
	return func(token string) {
		b, _ := json.Marshal(token)
		fmt.Fprintf(w, "data: %s\n\n", b)
		f.Flush()
	}
}

func sseError(w http.ResponseWriter, f http.Flusher, err error) {
	fmt.Fprintf(w, "data: [ERROR] %s\n\n", err.Error())
	f.Flush()
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// titleFromMarkdown extracts the text of the first # heading, or returns "".
func titleFromMarkdown(md string) string {
	for _, line := range strings.Split(md, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

// deriveTitle makes a short human title from the question/topic.
func deriveTitle(q string) string {
	q = strings.TrimSpace(q)
	q = strings.TrimSuffix(q, "?")
	if len(q) > 80 {
		q = q[:80] + "…"
	}
	if q == "" {
		return "Untitled"
	}
	return strings.ToUpper(q[:1]) + q[1:]
}

// normalizeMermaid coerces an LLM's diagram output into a single, cleanly
// fenced ```mermaid block. Models (especially smaller local ones) often wrap
// the diagram in prose or use a bare/mislabeled fence, which the document
// renderer won't draw. We pull out the diagram body and re-fence it.
func normalizeMermaid(out string) string {
	body := strings.TrimSpace(out)
	// If the output contains a fenced block, use its contents.
	if i := strings.Index(body, "```"); i >= 0 {
		rest := body[i+3:]
		// Drop the rest of the opening fence line (an optional language label).
		if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
			rest = rest[nl+1:]
		} else {
			rest = ""
		}
		if j := strings.Index(rest, "```"); j >= 0 {
			rest = rest[:j]
		}
		body = strings.TrimSpace(rest)
	}
	return "```mermaid\n" + body + "\n```"
}
