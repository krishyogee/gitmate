package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/krishyogee/gitmate/internal/config"
	"github.com/krishyogee/gitmate/internal/observability"
)

var ErrAllProvidersFailed = errors.New("all AI providers failed")
var ErrNoAPIKey = errors.New("no API key configured")

type Provider struct {
	Name    string
	Model   string
	APIKey  string
	BaseURL string
}

type Client struct {
	cfg       *config.Config
	logger    *observability.Logger
	providers []Provider
	current   int
	httpc     *http.Client
}

func NewClient(cfg *config.Config, logger *observability.Logger) *Client {
	c := &Client{cfg: cfg, logger: logger,
		httpc: &http.Client{Timeout: 90 * time.Second}}
	c.buildProviders()
	return c
}

func (c *Client) buildProviders() {
	c.providers = nil
	add := func(name, model, key, base string) {
		if key != "" {
			c.providers = append(c.providers, Provider{
				Name: name, Model: model, APIKey: key, BaseURL: base,
			})
		}
	}
	if k := os.Getenv("ANTHROPIC_API_KEY"); k != "" {
		add("anthropic", c.cfg.Models.Planning, k, "https://api.anthropic.com/v1/messages")
		add("anthropic", c.cfg.Models.Drafting, k, "https://api.anthropic.com/v1/messages")
		add("anthropic", c.cfg.Models.Fallback, k, "https://api.anthropic.com/v1/messages")
	}
	if k := os.Getenv("OPENAI_API_KEY"); k != "" {
		add("openai", "gpt-4o", k, "https://api.openai.com/v1/chat/completions")
		add("openai", "gpt-4o-mini", k, "https://api.openai.com/v1/chat/completions")
	}
	if k := os.Getenv("GROQ_API_KEY"); k != "" {
		add("groq", "llama-3.3-70b-versatile", k, "https://api.groq.com/openai/v1/chat/completions")
	}
}

func (c *Client) HasProvider() bool { return len(c.providers) > 0 }

func (c *Client) RotateModel() {
	if len(c.providers) == 0 {
		return
	}
	c.current = (c.current + 1) % len(c.providers)
}

func (c *Client) routeByTask(taskType string) int {
	if len(c.providers) == 0 {
		return -1
	}
	switch taskType {
	case "planning", "conflict_analysis":
		for i, p := range c.providers {
			if isStrong(p.Model) {
				return i
			}
		}
	case "commit_draft", "pr_draft":
		for i, p := range c.providers {
			if isFast(p.Model) {
				return i
			}
		}
	}
	return c.current
}

func isStrong(model string) bool {
	m := strings.ToLower(model)
	return strings.Contains(m, "opus") || strings.Contains(m, "gpt-4o") && !strings.Contains(m, "mini") || strings.Contains(m, "sonnet")
}

func isFast(model string) bool {
	m := strings.ToLower(model)
	return strings.Contains(m, "haiku") || strings.Contains(m, "mini") || strings.Contains(m, "llama")
}

func (c *Client) Complete(ctx context.Context, system, user, taskType string) (string, error) {
	if len(c.providers) == 0 {
		return "", ErrNoAPIKey
	}
	idx := c.routeByTask(taskType)
	if idx < 0 {
		idx = 0
	}

	var lastErr error
	for attempt := 0; attempt < len(c.providers)+1; attempt++ {
		p := c.providers[idx]
		start := time.Now()
		out, in, outTokens, err := c.callProvider(ctx, p, system, user)
		latency := time.Since(start).Milliseconds()
		c.logger.LogAICall(p.Name, p.Model, taskType, in, outTokens, latency, err)
		if err == nil {
			c.current = idx
			return out, nil
		}
		lastErr = err
		idx = (idx + 1) % len(c.providers)
	}
	return "", fmt.Errorf("%w: %v", ErrAllProvidersFailed, lastErr)
}

func (c *Client) callProvider(ctx context.Context, p Provider, system, user string) (string, int, int, error) {
	switch p.Name {
	case "anthropic":
		return c.callAnthropic(ctx, p, system, user)
	case "openai", "groq":
		return c.callOpenAILike(ctx, p, system, user)
	}
	return "", 0, 0, fmt.Errorf("unknown provider: %s", p.Name)
}

type anthropicReq struct {
	Model     string             `json:"model"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	MaxTokens int                `json:"max_tokens"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResp struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func (c *Client) callAnthropic(ctx context.Context, p Provider, system, user string) (string, int, int, error) {
	body := anthropicReq{
		Model:     p.Model,
		System:    system,
		MaxTokens: 2048,
		Messages:  []anthropicMessage{{Role: "user", Content: user}},
	}
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", p.BaseURL, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpc.Do(req)
	if err != nil {
		return "", 0, 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var out anthropicResp
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", 0, 0, fmt.Errorf("decode anthropic: %w (raw=%s)", err, string(raw))
	}
	if out.Error != nil {
		return "", 0, 0, fmt.Errorf("anthropic: %s", out.Error.Message)
	}
	if len(out.Content) == 0 {
		return "", 0, 0, fmt.Errorf("anthropic: empty content (status=%d, raw=%s)", resp.StatusCode, string(raw))
	}
	var txt strings.Builder
	for _, c := range out.Content {
		if c.Type == "text" {
			txt.WriteString(c.Text)
		}
	}
	return txt.String(), out.Usage.InputTokens, out.Usage.OutputTokens, nil
}

type openaiReq struct {
	Model    string          `json:"model"`
	Messages []openaiMessage `json:"messages"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResp struct {
	Choices []struct {
		Message openaiMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *Client) callOpenAILike(ctx context.Context, p Provider, system, user string) (string, int, int, error) {
	body := openaiReq{
		Model: p.Model,
		Messages: []openaiMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", p.BaseURL, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	resp, err := c.httpc.Do(req)
	if err != nil {
		return "", 0, 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out openaiResp
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", 0, 0, fmt.Errorf("decode openai: %w", err)
	}
	if out.Error != nil {
		return "", 0, 0, fmt.Errorf("%s: %s", p.Name, out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", 0, 0, fmt.Errorf("%s: empty choices (status=%d)", p.Name, resp.StatusCode)
	}
	return out.Choices[0].Message.Content, out.Usage.PromptTokens, out.Usage.CompletionTokens, nil
}
