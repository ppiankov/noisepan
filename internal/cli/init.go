package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ppiankov/noisepan/internal/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create config directory with example files",
	RunE:  initAction,
}

func initAction(_ *cobra.Command, _ []string) error {
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	created := 0

	configPath := filepath.Join(configDir, config.DefaultConfigFile)
	wrote, err := writeIfNotExists(configPath, []byte(exampleConfig))
	if err != nil {
		return err
	}
	if wrote {
		created++
	}

	tastePath := filepath.Join(configDir, config.DefaultTasteFile)
	wrote, err = writeIfNotExists(tastePath, []byte(exampleTaste))
	if err != nil {
		return err
	}
	if wrote {
		created++
	}

	if created == 0 {
		fmt.Printf("Config directory %s already initialized.\n", configDir)
	} else {
		fmt.Printf("Initialized %s with %d config files.\n", configDir, created)
	}
	return nil
}

// writeIfNotExists writes data to path if the file does not exist.
// Returns true if the file was created.
func writeIfNotExists(path string, data []byte) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("  exists: %s\n", path)
		return false, nil
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Printf("  created: %s\n", path)
	return true, nil
}

const exampleConfig = `# noisepan configuration

sources:
  telegram:
    api_id_env: TELEGRAM_API_ID
    api_hash_env: TELEGRAM_API_HASH
    session_dir: .noisepan/session
    channels:
      - "@your_channel_here"
  rss:
    feeds: []
    # - "https://example.com/feed.xml"
  reddit:
    subreddits: []
    # - "devops"
    # - "kubernetes"

storage:
  path: .noisepan/noisepan.db
  retain_days: 30

digest:
  timezone: "UTC"
  top_n: 7
  include_skims: 5
  since: 24h

summarize:
  mode: heuristic

privacy:
  store_full_text: false
  redact:
    enabled: false
    patterns: []
`

const exampleTaste = `# noisepan taste profile

weights:
  high_signal:
    "cve": 5
    "incident": 4
    "postmortem": 4
    "kubernetes": 3
    "breaking change": 5
    "outage": 4
    "zero-day": 5
  low_signal:
    "hiring": -3
    "webinar": -4
    "sponsor": -3
    "subscribe": -3

labels:
  critical:
    - "cve"
    - "zero-day"
  ops:
    - "kubernetes"
  incidents:
    - "postmortem"
    - "outage"
    - "incident"

rules:
  - if:
      contains_any: ["expired", "certificate"]
    then:
      score_add: 4
      labels: ["ops", "certs"]
  - if:
      contains_any: ["CVE-", "zero-day"]
    then:
      score_add: 5
      labels: ["critical"]

thresholds:
  read_now: 7
  skim: 3
  ignore: 0
`
