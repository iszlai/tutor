package main

import (
	"encoding/json"
	"errors"
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

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
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
