package summarize

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultEndpoint = "https://api.openai.com/v1/chat/completions"
	httpTimeout     = 30 * time.Second
	systemPrompt    = "Summarize for senior DevOps engineer. Focus on: breaking changes, incidents, security, architectural shifts. Max 4 bullets. Return only bullet points, one per line, starting with -"
)

// LLMSummarizer sends post text to an OpenAI-compatible API for summarization.
// Falls back to the provided heuristic summarizer on any error.
type LLMSummarizer struct {
	apiKey    string
	model     string
	maxTokens int
	endpoint  string
	fallback  Summarizer
	client    *http.Client
}

// NewLLM creates an LLM summarizer with a heuristic fallback.
func NewLLM(apiKey, model string, maxTokens int, fallback Summarizer) *LLMSummarizer {
	return &LLMSummarizer{
		apiKey:    apiKey,
		model:     model,
		maxTokens: maxTokens,
		endpoint:  defaultEndpoint,
		fallback:  fallback,
		client:    &http.Client{Timeout: httpTimeout},
	}
}

// Summarize calls the LLM API and parses the response into bullets.
// Links and CVEs are extracted via heuristic (LLM doesn't return structured data).
// On any error, falls back to the heuristic summarizer.
func (l *LLMSummarizer) Summarize(text string) Summary {
	bullets, err := l.callAPI(text)
	if err != nil {
		fmt.Fprintf(io.Discard, "llm summarize: %v\n", err)
		return l.fallback.Summarize(text)
	}

	if len(bullets) == 0 {
		return l.fallback.Summarize(text)
	}

	// Extract links and CVEs via heuristic
	links := urlRe.FindAllString(text, -1)
	cves := cveRe.FindAllString(text, -1)

	return Summary{
		Bullets: bullets,
		Links:   links,
		CVEs:    cves,
	}
}

func (l *LLMSummarizer) callAPI(text string) ([]string, error) {
	reqBody := chatRequest{
		Model: l.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: text},
		},
		MaxTokens: l.maxTokens,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, l.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+l.apiKey)

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api returned status %d", resp.StatusCode)
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("empty choices in response")
	}

	return parseBullets(chatResp.Choices[0].Message.Content), nil
}

// parseBullets extracts lines starting with "-" from LLM output.
func parseBullets(content string) []string {
	var bullets []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			bullets = append(bullets, strings.TrimPrefix(line, "- "))
		} else if strings.HasPrefix(line, "-") {
			bullets = append(bullets, strings.TrimPrefix(line, "-"))
		}
	}
	return bullets
}

type chatRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
}
