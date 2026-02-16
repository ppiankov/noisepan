package summarize

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func mockServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

func llmWithServer(url string) *LLMSummarizer {
	fallback := &HeuristicSummarizer{}
	s := NewLLM("test-key", "gpt-4", 200, fallback)
	s.endpoint = url
	return s
}

func respondJSON(w http.ResponseWriter, content string) {
	resp := chatResponse{
		Choices: []chatChoice{
			{Message: chatMessage{Role: "assistant", Content: content}},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func TestLLM_SuccessfulResponse(t *testing.T) {
	srv := mockServer(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type = %q", r.Header.Get("Content-Type"))
		}

		respondJSON(w, "- Critical CVE found in libfoo\n- Patch available in v2.1.0\n- No workaround exists")
	})
	defer srv.Close()

	s := llmWithServer(srv.URL)
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
	srv := mockServer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer srv.Close()

	s := llmWithServer(srv.URL)
	result := s.Summarize("some text about kubernetes")

	// Should fall back to heuristic
	if len(result.Bullets) == 0 {
		t.Fatal("expected fallback bullets")
	}
}

func TestLLM_Timeout(t *testing.T) {
	srv := mockServer(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
		respondJSON(w, "- too late")
	})
	defer srv.Close()

	s := llmWithServer(srv.URL)
	s.client.Timeout = 100 * time.Millisecond

	result := s.Summarize("some text")

	// Should fall back to heuristic
	if len(result.Bullets) == 0 {
		t.Fatal("expected fallback bullets")
	}
}

func TestLLM_EmptyChoices(t *testing.T) {
	srv := mockServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(chatResponse{Choices: []chatChoice{}})
	})
	defer srv.Close()

	s := llmWithServer(srv.URL)
	result := s.Summarize("some text")

	// Should fall back to heuristic
	if len(result.Bullets) == 0 {
		t.Fatal("expected fallback bullets")
	}
}

func TestLLM_MalformedJSON(t *testing.T) {
	srv := mockServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{{{not json"))
	})
	defer srv.Close()

	s := llmWithServer(srv.URL)
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
