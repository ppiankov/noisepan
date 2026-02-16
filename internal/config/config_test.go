package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeTestYAML(t *testing.T, dir, filename, content string) string {
	t.Helper()
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test yaml: %v", err)
	}
	return path
}

// --- Load tests ---

func TestLoad_FullConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TEST_TG_ID", "12345")
	t.Setenv("TEST_TG_HASH", "abcdef")
	t.Setenv("TEST_LLM_KEY", "sk-secret")

	writeTestYAML(t, dir, DefaultConfigFile, `
sources:
  telegram:
    api_id_env: TEST_TG_ID
    api_hash_env: TEST_TG_HASH
    session_dir: .noisepan/session
    channels:
      - "@test_channel"
storage:
  path: custom.db
  retain_days: 60
digest:
  timezone: "America/New_York"
  top_n: 10
  include_skims: 3
  since: 48h
summarize:
  mode: llm
  llm:
    provider: openai
    model: gpt-4.1-mini
    api_key_env: TEST_LLM_KEY
    max_tokens_per_post: 300
privacy:
  store_full_text: true
  redact:
    enabled: true
    patterns:
      - "(?i)token"
`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// Sources
	if cfg.Sources.Telegram.APIID != "12345" {
		t.Errorf("telegram api_id = %q, want 12345", cfg.Sources.Telegram.APIID)
	}
	if cfg.Sources.Telegram.APIHash != "abcdef" {
		t.Errorf("telegram api_hash = %q, want abcdef", cfg.Sources.Telegram.APIHash)
	}
	if cfg.Sources.Telegram.SessionDir != ".noisepan/session" {
		t.Errorf("session_dir = %q", cfg.Sources.Telegram.SessionDir)
	}
	if len(cfg.Sources.Telegram.Channels) != 1 || cfg.Sources.Telegram.Channels[0] != "@test_channel" {
		t.Errorf("channels = %v", cfg.Sources.Telegram.Channels)
	}

	// Storage
	if cfg.Storage.Path != "custom.db" {
		t.Errorf("storage path = %q, want custom.db", cfg.Storage.Path)
	}
	if cfg.Storage.RetainDays != 60 {
		t.Errorf("retain_days = %d, want 60", cfg.Storage.RetainDays)
	}

	// Digest
	if cfg.Digest.Timezone != "America/New_York" {
		t.Errorf("timezone = %q", cfg.Digest.Timezone)
	}
	if cfg.Digest.TopN != 10 {
		t.Errorf("top_n = %d, want 10", cfg.Digest.TopN)
	}
	if cfg.Digest.IncludeSkims != 3 {
		t.Errorf("include_skims = %d, want 3", cfg.Digest.IncludeSkims)
	}
	if cfg.Digest.Since.Duration != 48*time.Hour {
		t.Errorf("since = %v, want 48h", cfg.Digest.Since.Duration)
	}

	// Summarize
	if cfg.Summarize.Mode != "llm" {
		t.Errorf("mode = %q, want llm", cfg.Summarize.Mode)
	}
	if cfg.Summarize.LLM.APIKey != "sk-secret" {
		t.Errorf("llm api_key = %q, want sk-secret", cfg.Summarize.LLM.APIKey)
	}
	if cfg.Summarize.LLM.MaxTokensPerPost != 300 {
		t.Errorf("max_tokens = %d, want 300", cfg.Summarize.LLM.MaxTokensPerPost)
	}

	// Privacy
	if !cfg.Privacy.StoreFullText {
		t.Error("store_full_text = false, want true")
	}
	if !cfg.Privacy.Redact.Enabled {
		t.Error("redact.enabled = false, want true")
	}
	if len(cfg.Privacy.Redact.Patterns) != 1 {
		t.Errorf("redact patterns = %v", cfg.Privacy.Redact.Patterns)
	}
}

func TestLoad_DefaultsApplied(t *testing.T) {
	dir := t.TempDir()
	writeTestYAML(t, dir, DefaultConfigFile, `
sources:
  telegram:
    channels:
      - "@ch"
`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.Storage.Path != DefaultStoragePath {
		t.Errorf("storage.path = %q, want %q", cfg.Storage.Path, DefaultStoragePath)
	}
	if cfg.Storage.RetainDays != DefaultRetainDays {
		t.Errorf("retain_days = %d, want %d", cfg.Storage.RetainDays, DefaultRetainDays)
	}
	if cfg.Digest.TopN != DefaultTopN {
		t.Errorf("top_n = %d, want %d", cfg.Digest.TopN, DefaultTopN)
	}
	if cfg.Digest.IncludeSkims != DefaultIncludeSkims {
		t.Errorf("include_skims = %d, want %d", cfg.Digest.IncludeSkims, DefaultIncludeSkims)
	}
	if cfg.Digest.Since.Duration != DefaultSince {
		t.Errorf("since = %v, want %v", cfg.Digest.Since.Duration, DefaultSince)
	}
	if cfg.Digest.Timezone != DefaultTimezone {
		t.Errorf("timezone = %q, want %q", cfg.Digest.Timezone, DefaultTimezone)
	}
	if cfg.Summarize.Mode != DefaultSummarizeMode {
		t.Errorf("mode = %q, want %q", cfg.Summarize.Mode, DefaultSummarizeMode)
	}
}

func TestLoad_DurationParsing(t *testing.T) {
	dir := t.TempDir()
	writeTestYAML(t, dir, DefaultConfigFile, `
sources:
  telegram:
    channels: ["@ch"]
digest:
  since: 72h
`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Digest.Since.Duration != 72*time.Hour {
		t.Errorf("since = %v, want 72h", cfg.Digest.Since.Duration)
	}
}

func TestLoad_NoSources(t *testing.T) {
	dir := t.TempDir()
	writeTestYAML(t, dir, DefaultConfigFile, `
sources:
  telegram:
    channels: []
`)

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for no sources")
	}
	if want := "at least one source must be configured"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want containing %q", err, want)
	}
}

func TestLoad_RSSOnly(t *testing.T) {
	dir := t.TempDir()
	writeTestYAML(t, dir, DefaultConfigFile, `
sources:
  rss:
    feeds:
      - "https://example.com/feed.xml"
`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Sources.RSS.Feeds) != 1 {
		t.Errorf("rss feeds = %v, want 1 feed", cfg.Sources.RSS.Feeds)
	}
	if len(cfg.Sources.Telegram.Channels) != 0 {
		t.Errorf("telegram channels = %v, want empty", cfg.Sources.Telegram.Channels)
	}
}

func TestLoad_InvalidTimezone(t *testing.T) {
	dir := t.TempDir()
	writeTestYAML(t, dir, DefaultConfigFile, `
sources:
  telegram:
    channels: ["@ch"]
digest:
  timezone: "Not/AZone"
`)

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid timezone")
	}
	if want := "digest.timezone"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want containing %q", err, want)
	}
}

func TestLoad_InvalidSummarizeMode(t *testing.T) {
	dir := t.TempDir()
	writeTestYAML(t, dir, DefaultConfigFile, `
sources:
  telegram:
    channels: ["@ch"]
summarize:
  mode: quantum
`)

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
	if want := "unknown mode"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want containing %q", err, want)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load(t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if want := "read config"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want containing %q", err, want)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeTestYAML(t, dir, DefaultConfigFile, `{{{invalid`)

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for malformed yaml")
	}
	if want := "parse config"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want containing %q", err, want)
	}
}

func TestLoad_EmptyDir(t *testing.T) {
	_, err := Load("")
	if err == nil {
		t.Fatal("expected error for empty dir")
	}
	if want := "config dir is required"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want containing %q", err, want)
	}
}

func TestLoad_EnvVarResolution(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NP_TEST_KEY", "resolved-value")

	writeTestYAML(t, dir, DefaultConfigFile, `
sources:
  telegram:
    channels: ["@ch"]
summarize:
  mode: llm
  llm:
    api_key_env: NP_TEST_KEY
`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Summarize.LLM.APIKey != "resolved-value" {
		t.Errorf("api_key = %q, want resolved-value", cfg.Summarize.LLM.APIKey)
	}
}

func TestLoad_EnvVarMissing(t *testing.T) {
	dir := t.TempDir()
	writeTestYAML(t, dir, DefaultConfigFile, `
sources:
  telegram:
    api_id_env: NONEXISTENT_VAR_12345
    channels: ["@ch"]
`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Sources.Telegram.APIID != "" {
		t.Errorf("api_id = %q, want empty", cfg.Sources.Telegram.APIID)
	}
}

// --- LoadTaste tests ---

func TestLoadTaste_Full(t *testing.T) {
	dir := t.TempDir()
	path := writeTestYAML(t, dir, "taste.yaml", `
weights:
  high_signal:
    "cve": 5
    "kubernetes": 3
  low_signal:
    "hiring": -3
    "webinar": -4
labels:
  critical:
    - "cve"
    - "zero-day"
  ops:
    - "kubernetes"
rules:
  - if:
      contains_any: ["expired", "certificate"]
    then:
      score_add: 4
      labels: ["ops", "certs"]
  - if:
      contains_any: ["webinar"]
    then:
      score_add: -6
      labels: ["noise"]
thresholds:
  read_now: 7
  skim: 3
  ignore: 0
`)

	tp, err := LoadTaste(path)
	if err != nil {
		t.Fatalf("load taste: %v", err)
	}

	// Weights
	if tp.Weights.HighSignal["cve"] != 5 {
		t.Errorf("high_signal[cve] = %d, want 5", tp.Weights.HighSignal["cve"])
	}
	if tp.Weights.LowSignal["webinar"] != -4 {
		t.Errorf("low_signal[webinar] = %d, want -4", tp.Weights.LowSignal["webinar"])
	}

	// Labels
	if len(tp.Labels["critical"]) != 2 {
		t.Errorf("labels[critical] = %v", tp.Labels["critical"])
	}

	// Rules
	if len(tp.Rules) != 2 {
		t.Errorf("rules count = %d, want 2", len(tp.Rules))
	}
	if tp.Rules[0].Then.ScoreAdd != 4 {
		t.Errorf("rule[0] score_add = %d, want 4", tp.Rules[0].Then.ScoreAdd)
	}
	if len(tp.Rules[0].If.ContainsAny) != 2 {
		t.Errorf("rule[0] contains_any = %v", tp.Rules[0].If.ContainsAny)
	}

	// Thresholds
	if tp.Thresholds.ReadNow != 7 {
		t.Errorf("read_now = %d, want 7", tp.Thresholds.ReadNow)
	}
	if tp.Thresholds.Skim != 3 {
		t.Errorf("skim = %d, want 3", tp.Thresholds.Skim)
	}
}

func TestLoadTaste_InvalidThresholds(t *testing.T) {
	dir := t.TempDir()
	path := writeTestYAML(t, dir, "taste.yaml", `
thresholds:
  read_now: 3
  skim: 7
  ignore: 0
`)

	_, err := LoadTaste(path)
	if err == nil {
		t.Fatal("expected error for invalid thresholds")
	}
	if want := "read_now (3) must be greater than skim (7)"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want containing %q", err, want)
	}
}

func TestLoadTaste_InvalidThresholds_SkimLEIgnore(t *testing.T) {
	dir := t.TempDir()
	path := writeTestYAML(t, dir, "taste.yaml", `
thresholds:
  read_now: 7
  skim: 0
  ignore: 0
`)

	_, err := LoadTaste(path)
	if err == nil {
		t.Fatal("expected error for skim <= ignore")
	}
	if want := "skim (0) must be greater than ignore (0)"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want containing %q", err, want)
	}
}

func TestLoadTaste_MinimalValid(t *testing.T) {
	dir := t.TempDir()
	path := writeTestYAML(t, dir, "taste.yaml", `
thresholds:
  read_now: 7
  skim: 3
  ignore: 0
`)

	tp, err := LoadTaste(path)
	if err != nil {
		t.Fatalf("load taste: %v", err)
	}
	if tp.Weights.HighSignal != nil {
		t.Errorf("expected nil high_signal, got %v", tp.Weights.HighSignal)
	}
	if len(tp.Rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(tp.Rules))
	}
}

func TestLoadTaste_EmptyPath(t *testing.T) {
	_, err := LoadTaste("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
	if want := "taste profile path is required"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want containing %q", err, want)
	}
}

func TestLoadTaste_FileNotFound(t *testing.T) {
	_, err := LoadTaste(filepath.Join(t.TempDir(), "nope.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if want := "read taste profile"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want containing %q", err, want)
	}
}
