[![CI](https://github.com/ppiankov/noisepan/actions/workflows/ci.yml/badge.svg)](https://github.com/ppiankov/noisepan/actions/workflows/ci.yml)
[![Release](https://github.com/ppiankov/noisepan/actions/workflows/release.yml/badge.svg)](https://github.com/ppiankov/noisepan/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

# noisepan

Local-only signal extractor for noisy information streams. Telegram, RSS, Reddit sources. Deterministic keyword scoring, no ML. Terminal-first digest.

## Why This Exists

You are not lazy. You are overloaded.

The world produces more text than one brain can metabolize. Telegram channels, RSS feeds, Reddit threads — the signal-to-noise ratio is terrible and getting worse.

Noisepan is a gold pan for information: pour the stream through it, heavy signal stays, sand washes away.

## What This Is

- Reads posts from Telegram channels, RSS/Atom feeds, and Reddit (via RSS)
- Stores minimal metadata locally (SQLite, no cloud)
- Scores each post against your taste profile (keyword weights, rules, labels)
- Summarizes high-signal posts (heuristic by default, optional LLM via config)
- Prints a ranked terminal digest: Read Now / Skim / Ignore
- Outputs as terminal (ANSI), JSON, or Markdown
- Verifies source credibility via [entropia](https://github.com/ppiankov/entropia) integration
- Explains why each post was ranked (`noisepan explain`)

## What This Is NOT

- Not a Telegram client — does not send messages, does not join channels
- Not a chatbot — no interactive mode, no AI conversation
- Not ML/embedding-based — deterministic keyword + rule scoring
- Not a SaaS — no accounts, no cloud, no tracking
- Not a notification system — pull-based, runs when you run it
- Does not replace reading — reduces what you need to read

## Quick Start

### Install

```bash
brew install ppiankov/tap/noisepan
```

### Configure

```bash
noisepan --config ~/.noisepan init
```

Edit `~/.noisepan/config.yaml` — add your sources:

```yaml
sources:
  rss:
    feeds:
      - "https://www.reddit.com/r/devops/.rss"
      - "https://www.reddit.com/r/netsec/.rss"
      - "https://www.reddit.com/r/kubernetes/.rss"
  telegram:
    api_id_env: TELEGRAM_API_ID
    api_hash_env: TELEGRAM_API_HASH
    session_dir: /Users/you/.noisepan/session
    script: /Users/you/scripts/collector_telegram.py
    python_path: /Users/you/.noisepan/venv/bin/python
    channels:
      - "@your_channel"
```

Edit `~/.noisepan/taste.yaml` — tune your signal/noise weights.

See [docs/setup-guide.md](docs/setup-guide.md) for detailed setup instructions including Telegram authentication, venv setup, and shell configuration.

### Run

```bash
noisepan pull              # fetch new posts from sources
noisepan digest            # score + summarize + print
noisepan run               # pull + digest in one step
noisepan run --every 30m   # continuous mode
```

### Output

```
noisepan — 10 channels, 137 posts, since 1d

--- Read Now (6) ---

  [14] [critical] cybersecurity — Infosec exec sold eight zero-day exploit kits to Russia: DoJ
      https://www.reddit.com/r/cybersecurity/comments/.../

  [10] Kubernetes — Telescope - an open-source log viewer for ClickHouse, Docker and now Kubernetes
      https://www.reddit.com/r/kubernetes/comments/.../

  [9] [critical, llm] netsec — Prompt Injection Standardization: Text Techniques vs Intent
      https://www.reddit.com/r/netsec/comments/.../

--- Skim (5) ---

  [6] LocalLlama — SurrealDB 3.0 for agent memory
      https://www.reddit.com/r/LocalLLaMA/comments/.../
  [5] devops — What toolchain to use for alerts on logs?
      https://www.reddit.com/r/devops/comments/.../

Ignored: 117 posts (noise suppressed)
```

## Usage

| Command | Description |
|---------|-------------|
| `noisepan init` | Create config directory with example files |
| `noisepan pull` | Fetch new posts from configured sources |
| `noisepan digest` | Score, summarize, and print terminal digest |
| `noisepan run` | Pull + digest in one step |
| `noisepan run --every 30m` | Continuous mode with graceful shutdown |
| `noisepan verify` | Check source credibility of read_now posts via entropia |
| `noisepan explain <id>` | Show scoring breakdown for a post |
| `noisepan doctor` | Verify config, auth, and database health |
| `noisepan version` | Print version info |

| Flag | Applies to | Default | Description |
|------|-----------|---------|-------------|
| `--config DIR` | all | `.noisepan/` | Config directory path |
| `--since DUR` | digest, verify | `24h` | Time window |
| `--format FMT` | digest | `terminal` | Output: terminal, json, markdown |
| `--source SRC` | digest | all | Filter by source (rss, telegram) |
| `--channel CH` | digest | all | Filter by channel name |
| `--no-color` | digest, verify | false | Disable ANSI colors |
| `--every DUR` | run | off | Continuous mode interval |

## Architecture

```
cmd/noisepan/main.go       -- CLI entry point
internal/
  cli/                     -- Cobra commands (run, pull, digest, verify, explain, init, doctor)
  config/                  -- Config + taste profile loading (YAML)
  source/                  -- Source interface + implementations
    telegram.go            -- Telegram via Python/Telethon collector
    rss.go                 -- RSS/Atom feeds (gofeed)
    forgeplan.go           -- Local forge-plan script runner
  store/                   -- SQLite storage (posts, scores, dedup, retention)
  taste/                   -- Scoring engine: keywords, rules, labels, tiers
  summarize/               -- Heuristic + optional LLM summarizer
  digest/                  -- Terminal/JSON/Markdown formatters
  privacy/                 -- PII redaction (regex patterns)
```

## Taste Profile

Your taste profile defines what is signal and what is noise:

```yaml
weights:
  high_signal:
    "cve": 5
    "zero-day": 5
    "kubernetes": 3
    "llm": 3
    "devsecops": 3
  low_signal:
    "webinar": -4
    "hiring": -3

labels:
  critical:
    - "cve"
    - "zero-day"
  llm:
    - "llm"
    - "rag"
    - "ai agent"

rules:
  - if:
      contains_any: ["CVE-", "zero-day"]
    then:
      score_add: 5
      labels: ["critical"]

thresholds:
  read_now: 7    # score >= 7 → must read
  skim: 3        # score 3-6 → quick look
  ignore: 0      # score < 3 → skip
```

## Privacy

- All data stored locally in SQLite (`.noisepan/noisepan.db`)
- Full text storage is off by default — stores only 200-char snippets
- Configurable PII redaction patterns strip emails, tokens, API keys
- LLM summarization is optional and off by default (heuristic mode)
- No telemetry, no analytics, no cloud sync

## Known Limitations

- Telegram requires Python 3 + Telethon + one-time interactive login
- Reddit JSON API returns 403 — use RSS feeds instead (`/r/sub/.rss`)
- Heuristic summarizer is keyword-based (good enough for triage, not for deep understanding)
- LLM summarizer requires external API key and sends post text to the provider (set `summarize.mode: llm` in config)
- `verify` command requires [entropia](https://github.com/ppiankov/entropia) installed separately

## License

[MIT](LICENSE)
