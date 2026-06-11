package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// newID returns a sortable, collision-resistant id with a type prefix,
// e.g. "blk_01J4...". Not a strict ULID, but monotonic enough for our needs.
func newID(prefix string) string {
	ts := time.Now().UTC().UnixMilli()
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%012x%s", prefix, ts, hex.EncodeToString(b))
}

func nowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}
