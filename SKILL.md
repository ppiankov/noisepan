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
| `noisepan digest --output /tmp/d.md --format markdown` | Write digest to file |
| `noisepan digest --webhook https://hooks.slack.com/...` | POST JSON digest to webhook |
| `noisepan run` | Pull + digest in one step |
| `noisepan run --every 30m` | Continuous mode (pull + digest every 30m) |
| `noisepan stats` | Per-channel signal-to-noise ratios, scoring distribution |
| `noisepan stats --since 7d` | Stats for last 7 days |
| `noisepan stats --format json` | Machine-readable stats for scripted monitoring |
| `noisepan rescore` | Recompute all scores with current taste profile |
| `noisepan rescore --since 7d` | Rescore only last 7 days |
| `noisepan verify` | Check source credibility of read_now posts via entropia |
| `noisepan import feeds.opml` | Import RSS feeds from OPML file |
| `noisepan import feeds.opml --dry-run` | Preview what would be imported |
| `noisepan explain <post-id>` | Show scoring breakdown for one post |
| `noisepan doctor` | Health check: config, database, sources, feed health |
| `noisepan version` | Print version |

## Key Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config DIR` | `.noisepan/` | Config directory |
| `--since DUR` | `24h` / `30d` | Time window (e.g., `48h`, `7d`) |
| `--format FMT` | `terminal` | Output: `terminal`, `json`, `markdown` (stats: `terminal`, `json`) |
| `--source SRC` | all | Filter by source type |
| `--channel CH` | all | Filter by channel name |
| `--no-color` | false | Strip ANSI colors |
| `--output PATH` | stdout | Write digest to file |
| `--webhook URL` | off | POST digest JSON to URL |
| `--dry-run` | false | Preview import without modifying config |

## Agent Usage Pattern

For programmatic use, always use `--format json`:

```bash
noisepan pull && noisepan digest --format json --since 24h
```

### JSON Output Structure

```json
{
  "meta": { "channels": 28, "total_posts": 191, "since": "1d" },
  "trending": [
    { "keyword": "CVE-2026-1234", "channels": ["CISA", "Krebs", "BleepingComputer"] }
  ],
  "read_now": [
    {
      "source": "rss",
      "channel": "CISA",
      "url": "https://www.cisa.gov/...",
      "posted_at": "2026-02-20T10:00:00Z",
      "score": 10,
      "tier": "read_now",
      "labels": ["critical"],
      "headline": "First sentence of the post..."
    }
  ],
  "skims": [],
  "ignored": 164
}
```

### Parsing Examples

```bash
# Get all read_now items
noisepan digest --format json | jq '.read_now'

# Get URLs of critical posts
noisepan digest --format json | jq -r '.read_now[] | select(.labels | index("critical")) | .url'

# Get trending topics
noisepan digest --format json | jq '.trending'

# Save digest to file and post to webhook
noisepan digest --output ~/digest.json --format json --webhook https://hooks.slack.com/...
```

## Typical Workflow

1. **One-time setup:** `noisepan init` → edit config.yaml + taste.yaml
2. **Add feeds:** Edit `~/.noisepan/config.yaml` or `noisepan import feeds.opml`
3. **Tune weights:** Edit `~/.noisepan/taste.yaml` to match the operator's interests
4. **Daily digest:** `noisepan run` (or cron: `0 8 * * * noisepan run --output /tmp/digest.json --format json`)
5. **Check analytics:** `noisepan stats` to see signal-to-noise ratios per feed
6. **Verify top posts:** `noisepan verify` (requires `entropia` in PATH)
7. **Automated delivery:** `noisepan run --webhook https://hooks.slack.com/...`

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
