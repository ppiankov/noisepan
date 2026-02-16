package source

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

func jsonlFromMessages(t *testing.T, msgs []telegramMessage) io.Reader {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, m := range msgs {
		if err := enc.Encode(m); err != nil {
			t.Fatalf("encode test message: %v", err)
		}
	}
	return &buf
}

func TestParseJSONL_ValidMessages(t *testing.T) {
	msgs := []telegramMessage{
		{Channel: "devops_ru", MsgID: "100", Date: "2026-02-16T10:00:00Z", Text: "hello world", URL: "https://t.me/devops_ru/100"},
		{Channel: "k8s_news", MsgID: "200", Date: "2026-02-16T11:00:00Z", Text: "kubernetes 1.32 released", URL: "https://t.me/k8s_news/200"},
		{Channel: "devops_ru", MsgID: "101", Date: "2026-02-16T12:00:00Z", Text: "CVE-2026-1234 discovered", URL: "https://t.me/devops_ru/101"},
	}

	posts, err := parseJSONL(jsonlFromMessages(t, msgs))
	if err != nil {
		t.Fatalf("parseJSONL: %v", err)
	}

	if len(posts) != 3 {
		t.Fatalf("got %d posts, want 3", len(posts))
	}

	// Verify first post fields
	p := posts[0]
	if p.Source != "telegram" {
		t.Errorf("source = %q, want telegram", p.Source)
	}
	if p.Channel != "devops_ru" {
		t.Errorf("channel = %q, want devops_ru", p.Channel)
	}
	if p.ExternalID != "100" {
		t.Errorf("external_id = %q, want 100", p.ExternalID)
	}
	if p.Text != "hello world" {
		t.Errorf("text = %q, want hello world", p.Text)
	}
	if p.URL != "https://t.me/devops_ru/100" {
		t.Errorf("url = %q", p.URL)
	}

	want := time.Date(2026, 2, 16, 10, 0, 0, 0, time.UTC)
	if !p.PostedAt.Equal(want) {
		t.Errorf("posted_at = %v, want %v", p.PostedAt, want)
	}

	// Verify third post
	if posts[2].Text != "CVE-2026-1234 discovered" {
		t.Errorf("post[2].text = %q", posts[2].Text)
	}
}

func TestParseJSONL_EmptyInput(t *testing.T) {
	posts, err := parseJSONL(strings.NewReader(""))
	if err != nil {
		t.Fatalf("parseJSONL: %v", err)
	}
	if posts != nil {
		t.Errorf("got %d posts, want nil", len(posts))
	}
}

func TestParseJSONL_BlankLines(t *testing.T) {
	input := `{"channel":"ch","msg_id":"1","date":"2026-02-16T10:00:00Z","text":"first","url":"u1"}

{"channel":"ch","msg_id":"2","date":"2026-02-16T11:00:00Z","text":"second","url":"u2"}

`
	posts, err := parseJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parseJSONL: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("got %d posts, want 2", len(posts))
	}
	if posts[0].ExternalID != "1" {
		t.Errorf("post[0].id = %q, want 1", posts[0].ExternalID)
	}
	if posts[1].ExternalID != "2" {
		t.Errorf("post[1].id = %q, want 2", posts[1].ExternalID)
	}
}

func TestParseJSONL_InvalidJSON(t *testing.T) {
	input := `{"channel":"ch","msg_id":"1","date":"2026-02-16T10:00:00Z","text":"ok","url":"u"}
{not valid json}
`
	_, err := parseJSONL(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("error = %q, want containing 'line 2'", err)
	}
	if !strings.Contains(err.Error(), "invalid json") {
		t.Errorf("error = %q, want containing 'invalid json'", err)
	}
}

func TestParseJSONL_InvalidDate(t *testing.T) {
	input := `{"channel":"ch","msg_id":"1","date":"not-a-date","text":"ok","url":"u"}`

	_, err := parseJSONL(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid date")
	}
	if !strings.Contains(err.Error(), "line 1") {
		t.Errorf("error = %q, want containing 'line 1'", err)
	}
	if !strings.Contains(err.Error(), "invalid date") {
		t.Errorf("error = %q, want containing 'invalid date'", err)
	}
}

func TestParseJSONL_LargeMessage(t *testing.T) {
	// Generate text larger than the default 64 KiB scanner buffer
	largeText := strings.Repeat("a", 100_000)
	msg := telegramMessage{
		Channel: "ch",
		MsgID:   "1",
		Date:    "2026-02-16T10:00:00Z",
		Text:    largeText,
		URL:     "u",
	}

	posts, err := parseJSONL(jsonlFromMessages(t, []telegramMessage{msg}))
	if err != nil {
		t.Fatalf("parseJSONL: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("got %d posts, want 1", len(posts))
	}
	if len(posts[0].Text) != 100_000 {
		t.Errorf("text length = %d, want 100000", len(posts[0].Text))
	}
}

func TestNewTelegram_EmptyScriptPath(t *testing.T) {
	_, err := NewTelegram("", "id", "hash", "session", []string{"ch"})
	if err == nil {
		t.Fatal("expected error for empty script path")
	}
	if !strings.Contains(err.Error(), "script path is required") {
		t.Errorf("error = %q", err)
	}
}

func TestNewTelegram_EmptyChannels(t *testing.T) {
	_, err := NewTelegram("script.py", "id", "hash", "session", nil)
	if err == nil {
		t.Fatal("expected error for empty channels")
	}
	if !strings.Contains(err.Error(), "at least one channel") {
		t.Errorf("error = %q", err)
	}
}

func TestTelegramSource_Name(t *testing.T) {
	ts, err := NewTelegram("script.py", "id", "hash", "session", []string{"ch"})
	if err != nil {
		t.Fatalf("new telegram: %v", err)
	}
	if ts.Name() != "telegram" {
		t.Errorf("name = %q, want telegram", ts.Name())
	}
}

func TestTelegramSource_FetchNonexistentScript(t *testing.T) {
	ts, err := NewTelegram("/nonexistent/script.py", "id", "hash", "session", []string{"ch"})
	if err != nil {
		t.Fatalf("new telegram: %v", err)
	}

	_, err = ts.Fetch(time.Now().Add(-24 * time.Hour))
	if err == nil {
		t.Fatal("expected error for nonexistent script")
	}
	if !strings.Contains(err.Error(), "telegram:") {
		t.Errorf("error = %q, want containing 'telegram:'", err)
	}
}
