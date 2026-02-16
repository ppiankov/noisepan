[![CI](https://github.com/ppiankov/noisepan/actions/workflows/ci.yml/badge.svg)](https://github.com/ppiankov/noisepan/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

# noisepan

Extract signal from noisy information streams. Reads Telegram channels (and later RSS, Reddit, Medium), scores posts by relevance to your interests, and prints a concise digest in your terminal.

## Why This Exists

You are not lazy. You are overloaded.

The world produces more text than one brain can metabolize. Telegram channels, RSS feeds, Reddit threads, Medium posts — the signal-to-noise ratio is terrible and getting worse.

Noisepan is a gold pan for information: pour the stream through it, heavy signal stays, sand washes away.

## What This Is

- Reads posts from configured sources (Telegram channels first, more later)
- Stores minimal metadata locally (SQLite, no cloud)
- Scores each post against your taste profile (keyword weights, rules, labels)
- Summarizes high-signal posts (heuristic or optional LLM)
- Prints a ranked terminal digest: Read Now / Skim / Ignore
- Explains why each post was ranked (`noisepan explain`)

## What This Is NOT

- Not a Telegram client — does not send messages, does not join channels
- Not a chatbot — no interactive mode, no AI conversation
- Not a SaaS — no accounts, no cloud, no tracking
- Not a notification system — pull-based, runs when you run it
- Not a content aggregator — filters and ranks, does not collect everything
- Does not replace reading — reduces what you need to read

## Quick Start

### Install

```bash
# Homebrew
brew install ppiankov/tap/noisepan

# Go install
go install github.com/ppiankov/noisepan/cmd/noisepan@latest
```

### Configure

```bash
noisepan init    # creates .noisepan/ dir with example configs
# Edit .noisepan/config.yaml — add your Telegram channels
# Edit .noisepan/taste.yaml — tune your signal/noise weights
```

### Run

```bash
# Pull new posts + generate digest
noisepan run

# Or step by step:
noisepan pull              # fetch new posts from sources
noisepan digest            # score + summarize + print
noisepan digest --since 48h  # last 48 hours

# Why was this ranked high?
noisepan explain <post-id>
```

### Output

```
noisepan — 3 channels, 147 posts, 24h window

🔥 Read Now (3)
  1. K8s 1.36 removes dockershim shim-compat layer
     → impacts legacy node bootstrap, migration guide linked
     → score: 12  labels: [ops, breaking]

  2. Cert-manager CVE-2026-1182
     → affects ECDSA issuer rotation, patch 1.16.3 available
     → score: 10  labels: [critical, certs]

  3. ArgoCD 3.4 drift detection rewrite
     → fixes false positives in CRD-heavy clusters
     → score: 8   labels: [ops]

📎 Skim (5)
  4. New Linkerd stable release 2.17
  5. Terraform provider for Vault updated
  ...

⏭ Ignored: 139 posts (noise suppressed)
```

## Usage

| Command | Description |
|---------|-------------|
| `noisepan init` | Create config directory with example files |
| `noisepan pull` | Fetch new posts from configured sources |
| `noisepan digest` | Score, summarize, and print terminal digest |
| `noisepan run` | Pull + digest in one step |
| `noisepan explain <id>` | Show scoring breakdown for a post |
| `noisepan doctor` | Verify config, auth, and database health |
| `noisepan version` | Print version info |

| Flag | Default | Description |
|------|---------|-------------|
| `--config DIR` | `.noisepan/` | Config directory path |
| `--since DUR` | `24h` | Time window for digest |
| `--top N` | `7` | Max "Read Now" items |
| `--format` | `terminal` | Output format: terminal, json, markdown |
| `--taste FILE` | `.noisepan/taste.yaml` | Taste profile path |

## Architecture

```
cmd/noisepan/main.go       -- CLI entry point (minimal)
internal/
  cli/                     -- Cobra commands (run, pull, digest, explain, init, doctor)
  config/                  -- Config + taste profile loading
  source/                  -- Source interface + implementations
    telegram.go            -- Telegram channel reader
    rss.go                 -- RSS/Atom feed reader (future)
  store/                   -- SQLite storage for posts + scores
  taste/                   -- Scoring engine: keywords, rules, labels
  summarize/               -- Heuristic + optional LLM summarizer
  digest/                  -- Terminal/JSON/Markdown formatters
```

## Taste Profile

Your taste profile defines what is signal and what is noise:

```yaml
weights:
  high_signal:
    "cve": 5
    "postmortem": 4
    "kubernetes": 3
  low_signal:
    "webinar": -4
    "hiring": -3

thresholds:
  read_now: 7    # score >= 7 → must read
  skim: 3        # score 3-6 → quick look
  ignore: 0      # score < 3 → skip
```

See `configs/taste.example.yaml` for a complete DevOps-oriented profile.

## Privacy

- All data stored locally in SQLite (`.noisepan/noisepan.db`)
- Full text storage is off by default — stores only snippets and hashes
- Configurable PII redaction patterns strip tokens, passwords, API keys
- LLM summarization is optional and off by default (heuristic mode)
- No telemetry, no analytics, no cloud sync

## Known Limitations

- Telegram is the only source (RSS, Reddit, Medium planned)
- Heuristic summarizer is keyword-based (good enough for triage, not for deep understanding)
- LLM summarizer requires external API key and sends post text to the provider
- No daemon mode — runs once, exits (use cron/launchd for scheduling)
- Telegram private channels require personal API credentials

## Roadmap

- [ ] Telegram source with Telethon collector
- [ ] SQLite storage with deduplication
- [ ] Taste scoring engine
- [ ] Heuristic summarizer
- [ ] Terminal digest formatter
- [ ] `explain` command for scoring transparency
- [ ] RSS/Atom source
- [ ] LLM summarizer backend
- [ ] Reddit source
- [ ] Medium source
- [ ] Markdown/JSON output formats

## License

[MIT](LICENSE)
