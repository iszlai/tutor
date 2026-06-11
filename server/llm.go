package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// LLM is the provider seam. Claude today; swappable later. See docs/PLAN.md §5.4.
type LLM interface {
	Generate(ctx context.Context, system, prompt string) (string, error)
	// Stream calls onToken for each incremental text chunk and returns the full text.
	Stream(ctx context.Context, system, prompt string, onToken func(string)) (string, error)
	Name() string
	Model() string
}

// NewLLM picks a provider by environment, in priority order:
//  1. ANTHROPIC_API_KEY set            -> Claude (claude-opus-4-8)
//  2. TUTOR_LOCAL_LLM_URL set          -> local OpenAI-compatible model
//     (Ollama / llama.cpp / LM Studio; defaults to a Gemma 12B model)
//  3. otherwise                        -> deterministic mock (zero config)
func NewLLM() LLM {
	if key := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")); key != "" {
		return &claudeLLM{
			client: anthropic.NewClient(option.WithAPIKey(key)),
			model:  anthropic.ModelClaudeOpus4_8,
		}
	}
	if base := strings.TrimSpace(os.Getenv("TUTOR_LOCAL_LLM_URL")); base != "" {
		return &localLLM{
			baseURL: strings.TrimRight(base, "/"),
			model:   envOr("TUTOR_LOCAL_LLM_MODEL", "gemma3:12b"),
			http:    &http.Client{Timeout: 120 * time.Second},
		}
	}
	return &mockLLM{}
}

// ---- Claude ----------------------------------------------------------------

type claudeLLM struct {
	client anthropic.Client
	model  anthropic.Model
}

func (c *claudeLLM) Name() string  { return "anthropic" }
func (c *claudeLLM) Model() string { return string(c.model) }

func (c *claudeLLM) Generate(ctx context.Context, system, prompt string) (string, error) {
	return c.Stream(ctx, system, prompt, func(string) {})
}

func (c *claudeLLM) Stream(ctx context.Context, system, prompt string, onToken func(string)) (string, error) {
	s := c.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: 4096,
		System:    []anthropic.TextBlockParam{{Text: system}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	msg := anthropic.Message{}
	for s.Next() {
		ev := s.Current()
		if cbe, ok := ev.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
			if d, ok := cbe.Delta.AsAny().(anthropic.TextDelta); ok && d.Text != "" {
				onToken(d.Text)
			}
		}
		if err := msg.Accumulate(ev); err != nil {
			return "", err
		}
	}
	if err := s.Err(); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, block := range msg.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			b.WriteString(t.Text)
		}
	}
	return strings.TrimSpace(b.String()), nil
}

// ---- Local (OpenAI-compatible) ---------------------------------------------

// localLLM talks to any OpenAI-compatible /chat/completions endpoint. Point
// TUTOR_LOCAL_LLM_URL at an Ollama, llama.cpp, or LM Studio server running a
// local Gemma 12B model. Example (Ollama):
//
//	ollama pull gemma3:12b
//	export TUTOR_LOCAL_LLM_URL=http://localhost:11434/v1
//	export TUTOR_LOCAL_LLM_MODEL=gemma3:12b
type localLLM struct {
	baseURL string
	model   string
	http    *http.Client
}

func (l *localLLM) Name() string  { return "local" }
func (l *localLLM) Model() string { return l.model }

func (l *localLLM) Generate(ctx context.Context, system, prompt string) (string, error) {
	return l.Stream(ctx, system, prompt, func(string) {})
}

func (l *localLLM) Stream(ctx context.Context, system, prompt string, onToken func(string)) (string, error) {
	reqBody := map[string]interface{}{
		"model":       l.model,
		"stream":      true,
		"temperature": 0.3,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": prompt},
		},
	}
	if v := os.Getenv("TUTOR_LLM_NUM_CTX"); v != "" {
		var n int
		fmt.Sscan(v, &n)
		if n > 0 {
			reqBody["num_ctx"] = n
		}
	}
	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if k := os.Getenv("TUTOR_LOCAL_LLM_KEY"); k != "" {
		req.Header.Set("Authorization", "Bearer "+k)
	}
	// Use a client without a hard timeout; context handles cancellation.
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e struct{ Error struct{ Message string `json:"message"` } `json:"error"` }
		json.NewDecoder(resp.Body).Decode(&e) //nolint:errcheck
		return "", fmt.Errorf("local llm %d: %s", resp.StatusCode, e.Error.Message)
	}
	var full strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(data), &chunk) != nil || len(chunk.Choices) == 0 {
			continue
		}
		if t := chunk.Choices[0].Delta.Content; t != "" {
			full.WriteString(t)
			onToken(t)
		}
	}
	return strings.TrimSpace(full.String()), scanner.Err()
}

// ---- Mock ------------------------------------------------------------------

// mockLLM returns canned but realistic Markdown so the full interactive loop
// works offline. It keys off a few hints in the prompt.
type mockLLM struct{}

func (m *mockLLM) Name() string  { return "mock" }
func (m *mockLLM) Model() string { return "mock-tutor-1" }

func (m *mockLLM) Stream(_ context.Context, _, prompt string, onToken func(string)) (string, error) {
	text, err := m.Generate(context.Background(), "", prompt)
	if err != nil {
		return "", err
	}
	for _, word := range strings.Fields(text) {
		onToken(word + " ")
	}
	return text, nil
}

func (m *mockLLM) Generate(_ context.Context, _, prompt string) (string, error) {
	p := strings.ToLower(prompt)
	switch {
	case strings.Contains(p, "reply") || strings.Contains(p, "thread"):
		return mockReply(prompt), nil
	default:
		return mockDoc(prompt), nil
	}
}

func mockDoc(prompt string) string {
	return strings.TrimSpace(`
## Acceleration

**Acceleration** is the rate at which an object's velocity changes over time.
It is a vector quantity — it has both magnitude and direction.

The average acceleration over a time interval is:

$$a = \frac{\Delta v}{\Delta t} = \frac{v_f - v_i}{t}$$

where:

- $v_f$ is the **final velocity**
- $v_i$ is the **initial velocity**
- $t$ is the elapsed **time**

The SI unit is metres per second squared ($m/s^2$).

### Worked example

A car speeds up from $10\ m/s$ to $30\ m/s$ in $4$ seconds:

` + "```text\n" +
		`a = (30 - 10) / 4 = 5 m/s²` +
		"\n```" + `

So its velocity increases by 5 metres per second, every second.

> Note: a negative acceleration (deceleration) means the object is slowing
> down — the velocity change points opposite to the motion.
`)
}

func mockReply(prompt string) string {
	// Echo the selected text if we can find it, to feel anchored.
	sel := ""
	if i := strings.Index(prompt, "Selected text:"); i >= 0 {
		rest := prompt[i+len("Selected text:"):]
		sel = strings.TrimSpace(strings.SplitN(rest, "\n", 2)[0])
	}
	if sel != "" {
		return "Good question about **" + sel + "**. In this context it refers to " +
			"the variable's role in the equation — here it denotes velocity " +
			"(metres per second). Want me to expand this into its own page, or " +
			"insert a short definition into the document?"
	}
	return "Here's a bit more detail on that. Velocity is how fast position " +
		"changes with time; acceleration is how fast *velocity* changes with time."
}
