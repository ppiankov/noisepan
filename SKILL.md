---
name: noisepan
description: Local-only signal extractor for noisy information streams (RSS, Telegram, Reddit)
user-invocable: false
metadata: {"requires":{"bins":["noisepan"]}}
---

# noisepan — Signal Extraction

You have access to `noisepan`, a deterministic signal extractor for information streams. It reads posts from RSS, Telegram, and Reddit, scores them against a keyword-based taste profile, and outputs a ranked digest. No ML, no cloud, all local.

## Install

```bash
brew install ppiankov/tap/noisepan
```

## Setup

```bash
noisepan --config ~/.noisepan init
```

This creates `~/.noisepan/config.yaml` (sources) and `~/.noisepan/taste.yaml` (scoring weights). Edit both before first use.

### Minimal config.yaml (RSS only)

```yaml
sources:
  rss:
    feeds:
      - "https://www.reddit.com/r/devops/.rss"
      - "https://feeds.feedburner.com/TheHackersNews"
      - "https://www.cisa.gov/cybersecurity-advisories/all.xml"
```

### Minimal taste.yaml

```yaml
weights:
  high_signal:
    "cve": 5
    "zero-day": 5
    "kubernetes": 3
  low_signal:
    "webinar": -4
    "hiring": -3

thresholds:
  read_now: 7
  skim: 3
  ignore: 0
```

## Commands

| Command | What it does |
|---------|-------------|
| `noisepan pull` | Fetch new posts from all configured sources |
| `noisepan digest` | Score all posts, output ranked digest |
| `noisepan digest --format json` | Machine-readable JSON output |
| `noisepan digest --format markdown` | Markdown table output |
| `noisepan run` | Pull + digest in one step |
| `noisepan run --every 30m` | Continuous mode (pull + digest every 30m) |
| `noisepan verify` | Check source credibility of read_now posts via entropia |
| `noisepan explain <post-id>` | Show scoring breakdown for one post |
| `noisepan doctor` | Health check: config, database, sources |
| `noisepan version` | Print version |

## Key Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config DIR` | `.noisepan/` | Config directory |
| `--since DUR` | `24h` | Time window (e.g., `48h`, `7d`) |
| `--format FMT` | `terminal` | Output: `terminal`, `json`, `markdown` |
| `--source SRC` | all | Filter by source type |
| `--channel CH` | all | Filter by channel name |
| `--no-color` | false | Strip ANSI colors |

## Agent Usage Pattern

For programmatic use, always use `--format json`:

```bash
noisepan pull && noisepan digest --format json --since 24h
```

### JSON Output Structure

```json
[
  {
    "score": 10,
    "tier": "read_now",
    "labels": ["critical"],
    "source": "rss",
    "channel": "CISA",
    "title": "CISA Adds Three Known Exploited Vulnerabilities",
    "url": "https://www.cisa.gov/...",
    "summary": "First sentence of the post...",
    "posted_at": "2026-02-20T10:00:00Z"
  }
]
```

### Parsing Examples

```bash
# Get all read_now items
noisepan digest --format json | jq '[.[] | select(.tier == "read_now")]'

# Get URLs of critical posts
noisepan digest --format json | jq -r '.[] | select(.labels | index("critical")) | .url'

# Count by tier
noisepan digest --format json | jq 'group_by(.tier) | map({tier: .[0].tier, count: length})'
```

## Typical Workflow

1. **One-time setup:** `noisepan init` → edit config.yaml + taste.yaml
2. **Add feeds:** Edit `~/.noisepan/config.yaml` sources section
3. **Tune weights:** Edit `~/.noisepan/taste.yaml` to match the operator's interests
4. **Daily digest:** `noisepan run` (or cron: `0 8 * * * noisepan run --format json > /tmp/digest.json`)
5. **Verify top posts:** `noisepan verify` (requires `entropia` in PATH)

## Integration with entropia

The `verify` command calls `entropia scan <url> --json` on each read_now post. Install entropia first:

```bash
brew install ppiankov/tap/entropia
```

Posts from domains that can't be scanned (reddit.com, t.me) are skipped automatically.

## What noisepan Does NOT Do

- Does not send messages or interact with sources
- Does not use ML or embeddings — deterministic keyword scoring only
- Does not sync to cloud — all data in local SQLite
- Does not run as a daemon — pull-based, runs when invoked
- Does not replace reading — reduces what needs to be read

## Exit Codes

- `0` — success
- `1` — error (details on stderr)
