// Package llm is falcon's OPTIONAL, opt-in LLM enrichment layer.
//
// Design contract: the LLM is never required and never touches the deterministic
// core. The code graph (symbols/edges Parquet) is always built without it; LLM
// output lives in separate artifacts (e.g. community labels). It is local-first
// — the default backend is a local Ollama exposing an OpenAI-compatible API — so
// enrichment costs nothing, needs no API key, and keeps source private.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// Client talks to any OpenAI-compatible /chat/completions endpoint.
type Client struct {
	BaseURL string
	Model   string
	APIKey  string
	HTTP    *http.Client
}

// Default local Ollama OpenAI-compatible endpoint.
const (
	defaultBaseURL = "http://localhost:11434/v1"
	defaultModel   = "qwen2.5-coder:7b"
)

// FromEnv builds a client from environment, local-first:
//   - FALCON_LLM_BASE_URL (or OPENAI_BASE_URL) — default local Ollama
//   - FALCON_LLM_MODEL    (or OPENAI_MODEL)    — default qwen2.5-coder:7b
//   - OPENAI_API_KEY (optional; ignored by Ollama)
func FromEnv() *Client {
	base := firstNonEmpty(os.Getenv("FALCON_LLM_BASE_URL"), os.Getenv("OPENAI_BASE_URL"), defaultBaseURL)
	model := firstNonEmpty(os.Getenv("FALCON_LLM_MODEL"), os.Getenv("OPENAI_MODEL"), defaultModel)
	return &Client{
		BaseURL: strings.TrimRight(base, "/"),
		Model:   model,
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		HTTP:    &http.Client{Timeout: 120 * time.Second},
	}
}

type chatReq struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Temperature float64   `json:"temperature"`
	Stream      bool      `json:"stream"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResp struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete runs a single deterministic-ish (temperature 0) chat completion.
func (c *Client) Complete(ctx context.Context, system, user string) (string, error) {
	body, err := json.Marshal(chatReq{
		Model:       c.Model,
		Temperature: 0,
		Stream:      false,
		Messages: []message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm request to %s failed: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()

	var out chatResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("llm decode (status %d): %w", resp.StatusCode, err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("llm error: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("llm returned no choices (status %d)", resp.StatusCode)
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
