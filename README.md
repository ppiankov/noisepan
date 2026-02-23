[![CI](https://github.com/ppiankov/noisepan/actions/workflows/ci.yml/badge.svg)](https://github.com/ppiankov/noisepan/actions/workflows/ci.yml)
[![Release](https://github.com/ppiankov/noisepan/actions/workflows/release.yml/badge.svg)](https://github.com/ppiankov/noisepan/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![ANCC](https://img.shields.io/badge/ANCC-compliant-brightgreen)](https://ancc.dev)

# noisepan

Local-only signal extractor for noisy information streams. Telegram, RSS, Reddit sources. Deterministic keyword scoring, no ML. Terminal-first digest.

## Project Status

**Status: Beta** · **v0.4.0** · Pre-1.0

| Milestone | Status |
|-----------|--------|
| Core pipeline (pull/score/digest) | Complete |
| Sources (RSS, Telegram, forge-plan) | Complete |
| Output formats (terminal, JSON, markdown) | Complete |
| Stats, trending, rescore | Complete |
| Entropia verification integration | Complete |
| Test coverage >85% | Complete |
| CI pipeline (test/lint) | Complete |
| Homebrew distribution | Complete |
| Performance (parallel RSS, indexes, retry) | Complete |
| API stability guarantees | Partial |
| v1.0 release | Planned |

Pre-1.0: CLI flags and YAML config schemas may change between minor versions. JSON output structure is stable.

---

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
- Detects trending topics across channels (keyword appears in 3+ sources)
- Verifies source credibility via [entropia](https://github.com/ppiankov/entropia) integration
- Shows feed analytics and signal-to-noise ratios (`noisepan stats`)
- Imports feeds from OPML files (`noisepan import`)
- Routes digest to files or webhooks (`--output`, `--webhook`)
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

### Agent Integration

noisepan is designed to be used by autonomous agents without plugins or SDKs. Single binary, deterministic output, structured JSON, bounded jobs.

Agents: read [`SKILL.md`](SKILL.md) for install, commands, JSON parsing patterns, and workflow examples.

Key pattern for agents: `noisepan digest --format json` returns machine-parseable scored items.

Cross-tool integration: `noisepan verify` calls [entropia](https://github.com/ppiankov/entropia) automatically on read_now posts. Install both tools and the verification pipeline works out of the box — no wiring needed.

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

Digest ranks every post into tiers based on your taste profile:

```
noisepan — 28 channels, 191 posts, since 1d

--- Trending (appeared in 3+ sources) ---

  "CVE-2026-1234" — mentioned in 5 channels
    CISA, Krebs on Security, BleepingComputer, r/netsec, r/cybersecurity

--- Read Now (8) ---

  [10] [critical] CISA — CISA Adds Three Known Exploited Vulnerabilities
      https://www.cisa.gov/news-events/alerts/...

  [9] Krebs on Security — Patch Tuesday: Microsoft Fixes 63 Flaws
      https://krebsonsecurity.com/2026/...

  [8] [ops] Kubernetes Blog — Kubernetes v1.33: In-Place Pod Resize Graduates to GA
      https://kubernetes.io/blog/2026/...

--- Skim (5) ---

  [5] Cloudflare Blog — How we built automated canary deployments
      https://blog.cloudflare.com/...
  [4] Simon Willison — Using LLMs for structured data extraction
      https://simonwillison.net/2026/...

Ignored: 164 posts (noise suppressed)
```

### Verify

High scores don't mean the source is credible. A post can match all your keywords and still have no evidence behind it. The `verify` command checks read_now posts against [entropia](https://github.com/ppiankov/entropia) — a separate tool that evaluates how well claims are supported by available sources:

```bash
brew install ppiankov/tap/entropia   # one-time setup
noisepan verify
```

```
noisepan verify — 8 read_now posts, checking URLs...

--- Verification ---

  [10] [critical] CISA — CISA Adds Three Known Exploited Vulnerabilities
      https://www.cisa.gov/news-events/alerts/...
      entropia: support 88/100, confidence high, no conflict

  [9] Krebs on Security — Patch Tuesday: Microsoft Fixes 63 Flaws
      https://krebsonsecurity.com/2026/...
      entropia: support 72/100, confidence high, no conflict

  [8] [ops] Kubernetes Blog — Kubernetes v1.33: In-Place Pod Resize Graduates to GA
      https://kubernetes.io/blog/2026/...
      entropia: support 45/100, confidence medium, ⚠ conflict detected
```

A high score with `⚠ conflict detected` means entropia found contradictory evidence — the post scored well against your keywords but the underlying claims may not hold up. This is the signal to read critically rather than trust the headline.

Posts from domains that can't be scanned (reddit.com, t.me) are skipped with a reason. Verify works best with direct article feeds — blogs, advisories, vendor announcements — where entropia can actually fetch and evaluate the page.

## Usage

| Command | Description |
|---------|-------------|
| `noisepan init` | Create config directory with example files |
| `noisepan pull` | Fetch new posts from configured sources |
| `noisepan digest` | Score, summarize, and print terminal digest |
| `noisepan run` | Pull + digest in one step |
| `noisepan run --every 30m` | Continuous mode with graceful shutdown |
| `noisepan stats` | Show per-channel signal-to-noise ratios and scoring analytics |
| `noisepan stats --format json` | Machine-readable stats for scripted monitoring |
| `noisepan rescore` | Recompute all scores with current taste profile |
| `noisepan verify` | Check source credibility of read_now posts via entropia |
| `noisepan import <file.opml>` | Import RSS feeds from OPML file into config |
| `noisepan explain <id>` | Show scoring breakdown for a post |
| `noisepan doctor` | Verify config, auth, database health, and feed health |
| `noisepan version` | Print version info |

| Flag | Applies to | Default | Description |
|------|-----------|---------|-------------|
| `--config DIR` | all | `.noisepan/` | Config directory path |
| `--since DUR` | digest, stats, verify | `24h` / `30d` | Time window |
| `--format FMT` | digest, stats | `terminal` | Output: terminal, json (stats: terminal, json) |
| `--source SRC` | digest | all | Filter by source (rss, telegram) |
| `--channel CH` | digest | all | Filter by channel name |
| `--no-color` | digest, verify | false | Disable ANSI colors |
| `--every DUR` | run | off | Continuous mode interval |
| `--output PATH` | digest, run | stdout | Write digest to file |
| `--webhook URL` | digest, run | off | POST digest JSON to URL |
| `--dry-run` | import | false | Show what would be added |

## Architecture

```
cmd/noisepan/main.go       -- CLI entry point
internal/
  cli/                     -- Cobra commands (run, pull, digest, verify, stats, import, explain, init, doctor)
  config/                  -- Config + taste profile loading (YAML)
  source/                  -- Source interface + implementations
    telegram.go            -- Telegram via Python/Telethon collector
    rss.go                 -- RSS/Atom feeds (gofeed)
    forgeplan.go           -- Local forge-plan script runner
  store/                   -- SQLite storage (posts, scores, dedup, retention, channel stats)
  taste/                   -- Scoring engine: keywords, rules, labels, tiers, trending
  summarize/               -- Heuristic + optional LLM summarizer
  digest/                  -- Terminal/JSON/Markdown formatters (with trending section)
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
