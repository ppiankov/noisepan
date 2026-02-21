# Changelog

## [0.4.0]

- Rescore command: `noisepan rescore` recomputes all scores with current taste profile
- Stats JSON output: `noisepan stats --format json` for scripted monitoring
- Stats maturity indicator: channels with <30 days of data show "(Nd data)" to prevent premature pruning
- Database indexes on posted_at, text_hash, source/channel, tier (schema v2)
- Parallel RSS fetching: bounded goroutine pool (10 workers)
- RSS retry with exponential backoff (3 attempts, 1s/2s/4s on timeout/429/5xx)

## [0.3.0]

- Stats command: per-channel signal-to-noise ratios, scoring distribution, stale channel detection
- Trending detection: keywords appearing in 3+ channels highlighted at top of digest
- OPML feed import: `noisepan import feeds.opml` for bulk RSS onboarding
- Digest output routing: `--output` flag for file, `--webhook` flag for HTTP POST
- Feed health checks in doctor: stale feeds and all-ignored feeds warnings
- SKILL.md for agent integration with JSON output structure and workflow examples

## [0.2.2]

- E2E pipeline integration tests
- Watch mode with graceful shutdown (--every flag)
- Entropia verify command (source credibility for read_now posts)

## [0.2.1]

- Post URLs shown in terminal digest
- Browser-like User-Agent on RSS fetcher (Reddit RSS compatibility)
- Configurable telegram script path and python_path (venv support)

## [0.1.0]

- Telegram, RSS, Reddit, and forge-plan sources
- Keyword + rule-based taste scoring with tier assignment
- Heuristic and LLM summarizers
- Terminal, JSON, and Markdown digest output
- Cross-source post deduplication
- Data retention (retain_days pruning)
- Privacy controls (redaction patterns, full-text toggle)
- Source and channel filtering (--source, --channel flags)
- Doctor and explain commands
- Watch mode (--every flag)
