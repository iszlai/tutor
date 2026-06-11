package main

import (
	"bufio"
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	loadDotEnv(".env")

	dataDir := envOr("TUTOR_DATA_DIR", "./data")
	port := envOr("PORT", "8787")

	store, err := NewStore(dataDir)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	api := &API{store: store, llm: NewLLM()}

	log.Printf("tutor api on :%s  (provider=%s model=%s data=%s)",
		port, api.llm.Name(), api.llm.Model(), dataDir)
	if err := http.ListenAndServe(":"+port, api.routes()); err != nil {
		log.Fatal(err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// loadDotEnv loads KEY=VALUE lines from a .env file if present. Existing
// environment variables take precedence, so real env always wins.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.Trim(strings.TrimSpace(v), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			_ = os.Setenv(k, v)
		}
	}
}
