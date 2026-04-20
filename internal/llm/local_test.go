package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nlink-jp/data-agent/internal/config"
)

func TestLocalBackend_Chat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %s, want /chat/completions", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth = %q, want %q", r.Header.Get("Authorization"), "Bearer test-key")
		}

		var req openAIChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "test-model" {
			t.Errorf("model = %q, want %q", req.Model, "test-model")
		}
		if len(req.Messages) < 2 {
			t.Errorf("messages = %d, want at least 2 (system + user)", len(req.Messages))
		}
		if req.Stream {
			t.Error("stream should be false for Chat()")
		}

		json.NewEncoder(w).Encode(openAIChatResponse{
			Choices: []openAIChoice{{Message: openAIMessage{Content: "Hello from LLM"}}},
			Usage:   openAIUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		})
	}))
	defer server.Close()

	backend := NewLocalBackend(config.LocalLLMConfig{
		Endpoint: server.URL,
		Model:    "test-model",
		APIKey:   "test-key",
	})

	resp, err := backend.Chat(context.Background(), &ChatRequest{
		SystemPrompt: "You are helpful",
		Messages:     []Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if resp.Content != "Hello from LLM" {
		t.Errorf("content = %q, want %q", resp.Content, "Hello from LLM")
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("total_tokens = %d, want %d", resp.Usage.TotalTokens, 15)
	}
}

func TestLocalBackend_ChatStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openAIChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		if !req.Stream {
			t.Error("stream should be true for ChatStream()")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		chunks := []string{"Hello", " from", " stream"}
		for _, chunk := range chunks {
			data, _ := json.Marshal(openAIStreamChunk{
				Choices: []openAIStreamChoice{{Delta: openAIDelta{Content: chunk}}},
			})
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	backend := NewLocalBackend(config.LocalLLMConfig{
		Endpoint: server.URL,
		Model:    "test-model",
	})

	var tokens []string
	var doneCalled bool
	err := backend.ChatStream(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}, func(token string, done bool) {
		if done {
			doneCalled = true
		} else {
			tokens = append(tokens, token)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !doneCalled {
		t.Error("done callback was not called")
	}
	if len(tokens) != 3 {
		t.Errorf("received %d tokens, want 3", len(tokens))
	}
}

func TestLocalBackend_ChatError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model crashed", http.StatusInternalServerError)
	}))
	defer server.Close()

	backend := NewLocalBackend(config.LocalLLMConfig{Endpoint: server.URL, Model: "test"})
	_, err := backend.Chat(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestLocalBackend_ResponseJSON(t *testing.T) {
	var capturedReq openAIChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedReq)
		json.NewEncoder(w).Encode(openAIChatResponse{
			Choices: []openAIChoice{{Message: openAIMessage{Content: `{"key":"value"}`}}},
		})
	}))
	defer server.Close()

	backend := NewLocalBackend(config.LocalLLMConfig{Endpoint: server.URL, Model: "test"})
	_, err := backend.Chat(context.Background(), &ChatRequest{
		Messages:     []Message{{Role: "user", Content: "Return JSON"}},
		ResponseJSON: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if capturedReq.ResponseFormat == nil || capturedReq.ResponseFormat.Type != "json_object" {
		t.Error("response_format should be json_object when ResponseJSON is true")
	}
}

func TestLocalBackend_ResponseFormatFallback(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var req openAIChatRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.ResponseFormat != nil {
			// Reject response_format like some local LLM servers do
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"'response_format.type' must be 'json_schema' or 'text'"}`))
			return
		}

		// Second call without response_format succeeds
		json.NewEncoder(w).Encode(openAIChatResponse{
			Choices: []openAIChoice{{Message: openAIMessage{Content: `{"result":"ok"}`}}},
		})
	}))
	defer server.Close()

	backend := NewLocalBackend(config.LocalLLMConfig{Endpoint: server.URL, Model: "test"})
	resp, err := backend.Chat(context.Background(), &ChatRequest{
		Messages:     []Message{{Role: "user", Content: "Return JSON"}},
		ResponseJSON: true,
	})
	if err != nil {
		t.Fatalf("expected auto-fallback to succeed, got: %v", err)
	}
	if resp.Content != `{"result":"ok"}` {
		t.Errorf("content = %q, want JSON result", resp.Content)
	}
	if callCount != 2 {
		t.Errorf("calls = %d, want 2 (initial + fallback)", callCount)
	}
}

func TestLocalBackend_IsFormatUnsupportedError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"non-api error", fmt.Errorf("network error"), false},
		{"500 error", &apiError{StatusCode: 500, Body: "internal error"}, false},
		{"400 with response_format", &apiError{StatusCode: 400, Body: `{"error":"response_format not supported"}`}, true},
		{"422 with unsupported", &apiError{StatusCode: 422, Body: "unsupported parameter"}, true},
		{"400 unrelated", &apiError{StatusCode: 400, Body: "invalid model name"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isFormatUnsupportedError(tt.err)
			if got != tt.want {
				t.Errorf("isFormatUnsupportedError = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalBackend_Name(t *testing.T) {
	backend := NewLocalBackend(config.LocalLLMConfig{Model: "gemma-4-12b"})
	if backend.Name() != "local:gemma-4-12b" {
		t.Errorf("name = %q, want %q", backend.Name(), "local:gemma-4-12b")
	}
}
