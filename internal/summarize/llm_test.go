package summarize

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func llmWithTransport(rt roundTripFunc) *LLMSummarizer {
	fallback := &HeuristicSummarizer{}
	s := NewLLM("test-key", "gpt-4", 200, fallback)
	s.endpoint = "https://llm.test/v1/chat/completions"
	s.client = &http.Client{
		Timeout:   httpTimeout,
		Transport: rt,
	}
	return s
}

func responseJSON(content string) (*http.Response, error) {
	resp := chatResponse{
		Choices: []chatChoice{
			{Message: chatMessage{Role: "assistant", Content: content}},
		},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(string(b))),
	}, nil
}

func TestLLM_SuccessfulResponse(t *testing.T) {
	s := llmWithTransport(func(r *http.Request) (*http.Response, error) {
		// Verify request
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type = %q", r.Header.Get("Content-Type"))
		}
		return responseJSON("- Critical CVE found in libfoo\n- Patch available in v2.1.0\n- No workaround exists")
	})

	result := s.Summarize("CVE-2026-1234 found in libfoo. Patch in v2.1.0.")

	if len(result.Bullets) != 3 {
		t.Fatalf("bullets count = %d, want 3", len(result.Bullets))
	}
	if !strings.Contains(result.Bullets[0], "Critical CVE") {
		t.Errorf("bullet[0] = %q", result.Bullets[0])
	}

	// Heuristic extraction of CVEs from original text
	if len(result.CVEs) != 1 || result.CVEs[0] != "CVE-2026-1234" {
		t.Errorf("cves = %v", result.CVEs)
	}
}

func TestLLM_APIError(t *testing.T) {
	s := llmWithTransport(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil
	})

	result := s.Summarize("some text about kubernetes")

	// Should fall back to heuristic
	if len(result.Bullets) == 0 {
		t.Fatal("expected fallback bullets")
	}
}

func TestLLM_Timeout(t *testing.T) {
	s := llmWithTransport(func(req *http.Request) (*http.Response, error) {
		select {
		case <-time.After(2 * time.Second):
			return responseJSON("- too late")
		case <-req.Context().Done():
			return nil, errors.New("context canceled")
		}
	})
	s.client.Timeout = 100 * time.Millisecond

	result := s.Summarize("some text")

	// Should fall back to heuristic
	if len(result.Bullets) == 0 {
		t.Fatal("expected fallback bullets")
	}
}

func TestLLM_EmptyChoices(t *testing.T) {
	s := llmWithTransport(func(_ *http.Request) (*http.Response, error) {
		b, err := json.Marshal(chatResponse{Choices: []chatChoice{}})
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(string(b))),
		}, nil
	})
	result := s.Summarize("some text")

	// Should fall back to heuristic
	if len(result.Bullets) == 0 {
		t.Fatal("expected fallback bullets")
	}
}

func TestLLM_MalformedJSON(t *testing.T) {
	s := llmWithTransport(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader("{{{not json")),
		}, nil
	})
	result := s.Summarize("some text")

	// Should fall back to heuristic
	if len(result.Bullets) == 0 {
		t.Fatal("expected fallback bullets")
	}
}

func TestParseBullets(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"standard", "- First\n- Second\n- Third", 3},
		{"no dash prefix", "First bullet\nSecond bullet", 0},
		{"mixed", "Some intro\n- Actual bullet\nMore text\n- Another", 2},
		{"empty", "", 0},
		{"no space after dash", "-Compact bullet", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bullets := parseBullets(tt.input)
			if len(bullets) != tt.want {
				t.Errorf("parseBullets(%q) = %d bullets, want %d", tt.input, len(bullets), tt.want)
			}
		})
	}
}
