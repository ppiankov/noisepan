package source

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

const (
	sourceName    = "telegram"
	fetchTimeout  = 2 * time.Minute
	maxLineLength = 1 << 20 // 1 MiB per JSONL line
)

// TelegramSource fetches messages from Telegram channels via a Python helper script.
type TelegramSource struct {
	scriptPath string
	apiID      string
	apiHash    string
	sessionDir string
	channels   []string
}

// NewTelegram creates a Telegram source. The scriptPath must point to the
// collector_telegram.py script. API credentials and channels come from config.
func NewTelegram(scriptPath, apiID, apiHash, sessionDir string, channels []string) (*TelegramSource, error) {
	if strings.TrimSpace(scriptPath) == "" {
		return nil, errors.New("telegram: script path is required")
	}
	if len(channels) == 0 {
		return nil, errors.New("telegram: at least one channel is required")
	}

	return &TelegramSource{
		scriptPath: scriptPath,
		apiID:      apiID,
		apiHash:    apiHash,
		sessionDir: sessionDir,
		channels:   channels,
	}, nil
}

// Name returns "telegram".
func (ts *TelegramSource) Name() string {
	return sourceName
}

// Fetch invokes the Python collector script and parses JSONL output.
func (ts *TelegramSource) Fetch(since time.Time) ([]Post, error) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()

	args := []string{
		ts.scriptPath,
		"--api-id", ts.apiID,
		"--api-hash", ts.apiHash,
		"--session-dir", ts.sessionDir,
		"--channels", strings.Join(ts.channels, ","),
		"--since", since.UTC().Format(time.RFC3339),
	}

	cmd := exec.CommandContext(ctx, "python3", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("telegram: stdout pipe: %w", err)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("telegram: python3 not found: install Python 3 and Telethon to use telegram source")
		}
		return nil, fmt.Errorf("telegram: start collector: %w", err)
	}

	posts, parseErr := parseJSONL(stdout)

	if err := cmd.Wait(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return nil, fmt.Errorf("telegram: collector failed: %s", errMsg)
		}
		return nil, fmt.Errorf("telegram: collector failed: %w", err)
	}

	if parseErr != nil {
		return nil, fmt.Errorf("telegram: parse output: %w", parseErr)
	}

	return posts, nil
}

// telegramMessage is the JSONL schema emitted by the Python collector.
type telegramMessage struct {
	Channel string `json:"channel"`
	MsgID   string `json:"msg_id"`
	Date    string `json:"date"`
	Text    string `json:"text"`
	URL     string `json:"url"`
}

// parseJSONL reads JSONL from r and converts each line to a Post.
func parseJSONL(r io.Reader) ([]Post, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, maxLineLength), maxLineLength)

	var posts []Post
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var msg telegramMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			return nil, fmt.Errorf("line %d: invalid json: %w", lineNum, err)
		}

		postedAt, err := time.Parse(time.RFC3339, msg.Date)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid date %q: %w", lineNum, msg.Date, err)
		}

		posts = append(posts, Post{
			Source:     sourceName,
			Channel:    msg.Channel,
			ExternalID: msg.MsgID,
			Text:       msg.Text,
			URL:        msg.URL,
			PostedAt:   postedAt,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read jsonl: %w", err)
	}

	return posts, nil
}
