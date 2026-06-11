package main

import (
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
	// Stream and accumulate: keeps long one-pagers under the SDK HTTP timeout.
	stream := c.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: 4096,
		System:    []anthropic.TextBlockParam{{Text: system}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})

	msg := anthropic.Message{}
	for stream.Next() {
		if err := msg.Accumulate(stream.Current()); err != nil {
			return "", err
		}
	}
	if err := stream.Err(); err != nil {
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
	body, _ := json.Marshal(map[string]interface{}{
		"model":       l.model,
		"stream":      false,
		"temperature": 0.3,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": prompt},
		},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	// Some local servers ignore auth; send a dummy key if one is configured.
	if k := os.Getenv("TUTOR_LOCAL_LLM_KEY"); k != "" {
		req.Header.Set("Authorization", "Bearer "+k)
	}

	resp, err := l.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("local llm %d: %s", resp.StatusCode, out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("local llm returned no choices")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

// ---- Mock ------------------------------------------------------------------

// mockLLM returns canned but realistic Markdown so the full interactive loop
// works offline. It keys off a few hints in the prompt.
type mockLLM struct{}

func (m *mockLLM) Name() string  { return "mock" }
func (m *mockLLM) Model() string { return "mock-tutor-1" }

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
