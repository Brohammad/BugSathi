package openai

import (
	"bytes"
	"context"
	"encoding/base64"
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
	Content any    `json:"content"`
}

type contentPart struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *imageContent `json:"image_url,omitempty"`
}

type imageContent struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
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
	content, err := buildContent(in)
	if err != nil {
		return domain.AnalysisResult{}, err
	}
	prompt := buildPrompt(in)
	body, err := json.Marshal(chatReq{
		Model: a.cfg.Model,
		Messages: []chatMessage{
			{Role: "system", Content: "You are a QA engineer writing concise bug reports. Respond with JSON only."},
			{Role: "user", Content: append([]contentPart{{Type: "text", Text: prompt}}, content...)},
		},
		ResponseFormat: &respFormat{Type: "json_object"},
	})
	if err != nil {
		return domain.AnalysisResult{}, fmt.Errorf("encode openai request: %w", err)
	}

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

func buildContent(in domain.AnalysisInput) ([]contentPart, error) {
	if len(in.Frames) == 0 {
		return nil, fmt.Errorf("openai visual analysis requires at least one loaded frame")
	}
	content := make([]contentPart, 0, len(in.Frames)*2)
	for i, frame := range in.Frames {
		if len(frame.Data) == 0 {
			return nil, fmt.Errorf("frame %d is empty", i)
		}
		switch frame.MediaType {
		case "image/jpeg", "image/png", "image/webp":
		default:
			return nil, fmt.Errorf("frame %d has unsupported media type %q", i, frame.MediaType)
		}
		content = append(content,
			contentPart{
				Type: "text",
				Text: fmt.Sprintf(
					"Frame %d of %d (%s in chronological order):",
					i+1,
					len(in.Frames),
					frame.StorageKey,
				),
			},
			contentPart{
				Type: "image_url",
				ImageURL: &imageContent{
					URL:    "data:" + frame.MediaType + ";base64," + base64.StdEncoding.EncodeToString(frame.Data),
					Detail: "high",
				},
			},
		)
	}
	return content, nil
}

func buildPrompt(in domain.AnalysisInput) string {
	meta := "{}"
	if len(in.MetadataJSON) > 0 {
		meta = string(in.MetadataJSON)
	}
	return fmt.Sprintf(`Create a bug report from the attached chronological screen-recording frames.
recording_id: %s
project_id: %s
prompt_version: %s
frame_count: %d
client_metadata_json: %s

Ground every claim in visible frame evidence or supplied metadata. Do not invent
clicks, text, errors, expected behavior, or intermediate actions that are not
supported by the inputs.

Quote decisive visible error messages, status text, paths, identifiers, and
numeric values verbatim. Reproduction steps may include only actions directly
visible in the frames. When the triggering action is not visible, state
"Trigger unknown from provided frames" and describe only what can be observed.

Return JSON: {"title":"...","summary":"...","steps":["..."]}`,
		in.RecordingID, in.ProjectID, in.PromptVersion, len(in.Frames), meta)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
