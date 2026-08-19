package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Brohammad/BugSathi/internal/ai/domain"
)

type Config struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

type Analyzer struct {
	cfg    Config
	client *http.Client
}

func New(cfg Config) *Analyzer {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-4o-mini"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}
	return &Analyzer{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

type chatReq struct {
	Model          string        `json:"model"`
	Messages       []chatMessage `json:"messages"`
	ResponseFormat *respFormat   `json:"response_format,omitempty"`
}

type respFormat struct {
	Type string `json:"type"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type llmJSON struct {
	Title   string   `json:"title"`
	Summary string   `json:"summary"`
	Steps   []string `json:"steps"`
}

func (a *Analyzer) Analyze(ctx context.Context, in domain.AnalysisInput) (domain.AnalysisResult, error) {
	if a.cfg.APIKey == "" {
		return domain.AnalysisResult{}, fmt.Errorf("AI_API_KEY is required for openai provider")
	}
	prompt := buildPrompt(in)
	body, _ := json.Marshal(chatReq{
		Model: a.cfg.Model,
		Messages: []chatMessage{
			{Role: "system", Content: "You are a QA engineer writing concise bug reports. Respond with JSON only."},
			{Role: "user", Content: prompt},
		},
		ResponseFormat: &respFormat{Type: "json_object"},
	})

	url := strings.TrimRight(a.cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return domain.AnalysisResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+a.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := a.client.Do(req)
	if err != nil {
		return domain.AnalysisResult{}, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return domain.AnalysisResult{}, fmt.Errorf("openai status %d: %s", res.StatusCode, truncate(string(raw), 300))
	}
	var parsed chatResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return domain.AnalysisResult{}, err
	}
	if parsed.Error != nil {
		return domain.AnalysisResult{}, fmt.Errorf("openai: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return domain.AnalysisResult{}, fmt.Errorf("openai: empty choices")
	}
	var out llmJSON
	if err := json.Unmarshal([]byte(parsed.Choices[0].Message.Content), &out); err != nil {
		return domain.AnalysisResult{}, fmt.Errorf("parse llm json: %w", err)
	}
	return domain.AnalysisResult{
		Title:    out.Title,
		Summary:  out.Summary,
		Steps:    out.Steps,
		Provider: "openai",
		Model:    a.cfg.Model,
	}, nil
}

func buildPrompt(in domain.AnalysisInput) string {
	meta := "{}"
	if len(in.MetadataJSON) > 0 {
		meta = string(in.MetadataJSON)
	}
	keys := strings.Join(in.FrameKeys, ", ")
	// MVP: frame keys are sent as text in the prompt, not as image bytes (see ADR 0030).
	return fmt.Sprintf(`Create a bug report from this screen recording context.
recording_id: %s
project_id: %s
prompt_version: %s
frame_object_keys: %s
client_metadata_json: %s

Return JSON: {"title":"...","summary":"...","steps":["..."]}`,
		in.RecordingID, in.ProjectID, in.PromptVersion, keys, meta)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
