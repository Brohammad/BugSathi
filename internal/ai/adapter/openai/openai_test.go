package openai_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Brohammad/BugSathi/internal/ai/adapter/openai"
	"github.com/Brohammad/BugSathi/internal/ai/domain"
)

func TestAnalyzeSendsSelectedFramesAsMultimodalContent(t *testing.T) {
	requests := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization=%q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		requests <- body
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"title\":\"Broken checkout\",\"summary\":\"An inline error is visible.\",\"steps\":[\"Open checkout\"]}"}}]}`))
	}))
	defer server.Close()

	analyzer := openai.New(openai.Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "vision-test",
		Timeout: time.Second,
	})
	frames := []domain.FrameInput{
		{StorageKey: "frame-0.jpg", MediaType: "image/jpeg", Data: []byte{0xff, 0xd8, 0xff}},
		{StorageKey: "frame-4.png", MediaType: "image/png", Data: []byte{0x89, 0x50, 0x4e, 0x47}},
	}
	result, err := analyzer.Analyze(context.Background(), domain.AnalysisInput{
		RecordingID:   "recording-id",
		ProjectID:     "project-id",
		FrameKeys:     []string{"frame-0.jpg", "frame-4.png"},
		Frames:        frames,
		MetadataJSON:  json.RawMessage(`{"browser":"chrome"}`),
		PromptVersion: domain.PromptVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Title != "Broken checkout" || result.Provider != "openai" {
		t.Fatalf("result=%+v", result)
	}

	body := <-requests
	messages := mustSlice(t, body["messages"], "messages")
	if len(messages) != 2 {
		t.Fatalf("messages=%d want 2", len(messages))
	}
	user := mustMap(t, messages[1], "user message")
	content := mustSlice(t, user["content"], "user content")
	if len(content) != 3 {
		t.Fatalf("content items=%d want text+2 images", len(content))
	}
	text := mustMap(t, content[0], "text content")
	if text["type"] != "text" {
		t.Fatalf("first content=%v", text)
	}
	for i, frame := range frames {
		part := mustMap(t, content[i+1], "image content")
		if part["type"] != "image_url" {
			t.Fatalf("content[%d] type=%v", i+1, part["type"])
		}
		imageURL := mustMap(t, part["image_url"], "image_url")
		want := "data:" + frame.MediaType + ";base64," + base64.StdEncoding.EncodeToString(frame.Data)
		if imageURL["url"] != want {
			t.Fatalf("content[%d] url=%q want %q", i+1, imageURL["url"], want)
		}
	}
}

func TestAnalyzeRejectsMissingVisualFrames(t *testing.T) {
	analyzer := openai.New(openai.Config{
		BaseURL: "http://127.0.0.1:1",
		APIKey:  "test-key",
		Timeout: 50 * time.Millisecond,
	})
	_, err := analyzer.Analyze(context.Background(), domain.AnalysisInput{
		RecordingID: "recording-id",
		FrameKeys:   []string{"private/frame.jpg"},
	})
	if err == nil {
		t.Fatal("expected missing visual frames to be rejected before provider request")
	}
}

func mustMap(t *testing.T, value any, name string) map[string]any {
	t.Helper()
	out, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s=%T want object", name, value)
	}
	return out
}

func mustSlice(t *testing.T, value any, name string) []any {
	t.Helper()
	out, ok := value.([]any)
	if !ok {
		t.Fatalf("%s=%T want array", name, value)
	}
	return out
}
