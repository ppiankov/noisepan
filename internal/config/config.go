package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigFile    = "config.yaml"
	DefaultTasteFile     = "taste.yaml"
	DefaultStoragePath   = ".noisepan/noisepan.db"
	DefaultRetainDays    = 30
	DefaultTopN          = 7
	DefaultIncludeSkims  = 5
	DefaultSince         = 24 * time.Hour
	DefaultTimezone      = "UTC"
	DefaultSummarizeMode = "heuristic"
)

// Duration wraps time.Duration for YAML unmarshaling from strings like "24h".
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", s, err)
	}
	d.Duration = parsed
	return nil
}

type Config struct {
	Sources   SourcesConfig   `yaml:"sources"`
	Storage   StorageConfig   `yaml:"storage"`
	Digest    DigestConfig    `yaml:"digest"`
	Summarize SummarizeConfig `yaml:"summarize"`
	Privacy   PrivacyConfig   `yaml:"privacy"`
}

type SourcesConfig struct {
	Telegram  TelegramConfig  `yaml:"telegram"`
	RSS       RSSConfig       `yaml:"rss"`
	Reddit    RedditConfig    `yaml:"reddit"`
	HN        HNConfig        `yaml:"hn"`
	ForgePlan ForgePlanConfig `yaml:"forgeplan"`
}

type HNConfig struct {
	MinPoints int `yaml:"min_points"`
}

type ForgePlanConfig struct {
	Script string `yaml:"script"`
}

type RSSConfig struct {
	Feeds []string `yaml:"feeds"`
}

type RedditConfig struct {
	Subreddits []string `yaml:"subreddits"`
}

type TelegramConfig struct {
	APIIDEnv   string   `yaml:"api_id_env"`
	APIHashEnv string   `yaml:"api_hash_env"`
	SessionDir string   `yaml:"session_dir"`
	Channels   []string `yaml:"channels"`
	Script     string   `yaml:"script"`
	PythonPath string   `yaml:"python_path"`

	// Resolved from env vars at load time.
	APIID   string `yaml:"-"`
	APIHash string `yaml:"-"`
}

type StorageConfig struct {
	Path       string `yaml:"path"`
	RetainDays int    `yaml:"retain_days"`
}

type DigestConfig struct {
	Timezone     string   `yaml:"timezone"`
	TopN         int      `yaml:"top_n"`
	IncludeSkims int      `yaml:"include_skims"`
	Since        Duration `yaml:"since"`
}

type SummarizeConfig struct {
	Mode string    `yaml:"mode"`
	LLM  LLMConfig `yaml:"llm"`
}

type LLMConfig struct {
	Provider         string `yaml:"provider"`
	Model            string `yaml:"model"`
	APIKeyEnv        string `yaml:"api_key_env"`
	MaxTokensPerPost int    `yaml:"max_tokens_per_post"`

	// Resolved from env var at load time.
	APIKey string `yaml:"-"`
}

type PrivacyConfig struct {
	StoreFullText bool         `yaml:"store_full_text"`
	Redact        RedactConfig `yaml:"redact"`
}

type RedactConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Patterns []string `yaml:"patterns"`
}

// Load reads config.yaml from dir, applies defaults, resolves env vars, and validates.
func Load(dir string) (*Config, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("config dir is required")
	}

	path := filepath.Join(dir, DefaultConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	applyDefaults(&cfg)
	resolveEnv(&cfg)

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Storage.Path == "" {
		cfg.Storage.Path = DefaultStoragePath
	}
	if cfg.Storage.RetainDays == 0 {
		cfg.Storage.RetainDays = DefaultRetainDays
	}
	if cfg.Digest.TopN == 0 {
		cfg.Digest.TopN = DefaultTopN
	}
	if cfg.Digest.IncludeSkims == 0 {
		cfg.Digest.IncludeSkims = DefaultIncludeSkims
	}
	if cfg.Digest.Since.Duration == 0 {
		cfg.Digest.Since.Duration = DefaultSince
	}
	if cfg.Digest.Timezone == "" {
		cfg.Digest.Timezone = DefaultTimezone
	}
	if cfg.Summarize.Mode == "" {
		cfg.Summarize.Mode = DefaultSummarizeMode
	}
}

func resolveEnv(cfg *Config) {
	if cfg.Sources.Telegram.APIIDEnv != "" {
		cfg.Sources.Telegram.APIID = os.Getenv(cfg.Sources.Telegram.APIIDEnv)
	}
	if cfg.Sources.Telegram.APIHashEnv != "" {
		cfg.Sources.Telegram.APIHash = os.Getenv(cfg.Sources.Telegram.APIHashEnv)
	}
	if cfg.Summarize.LLM.APIKeyEnv != "" {
		cfg.Summarize.LLM.APIKey = os.Getenv(cfg.Summarize.LLM.APIKeyEnv)
	}
}

func validate(cfg *Config) error {
	hasTelegram := len(cfg.Sources.Telegram.Channels) > 0
	hasRSS := len(cfg.Sources.RSS.Feeds) > 0
	hasReddit := len(cfg.Sources.Reddit.Subreddits) > 0
	hasHN := cfg.Sources.HN.MinPoints > 0
	hasForgePlan := cfg.Sources.ForgePlan.Script != ""
	if !hasTelegram && !hasRSS && !hasReddit && !hasHN && !hasForgePlan {
		return errors.New("sources: at least one source must be configured")
	}

	if _, err := time.LoadLocation(cfg.Digest.Timezone); err != nil {
		return fmt.Errorf("digest.timezone: %w", err)
	}

	switch cfg.Summarize.Mode {
	case "heuristic", "llm":
		// valid
	default:
		return fmt.Errorf("summarize.mode: unknown mode %q (want heuristic or llm)", cfg.Summarize.Mode)
	}

	return nil
}
