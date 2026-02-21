# Work Orders — noisepan

Roadmap for noisepan development. Each work order is self-contained with scope, affected files, acceptance criteria, and implementation notes.

Status key: `[ ]` planned, `[~]` in progress, `[x]` done

---

## Phase 1: Core Pipeline

### WO-N01: SQLite storage layer

**Status:** `[x]` done
**Priority:** high — everything depends on persistent storage

### Summary

Create SQLite-backed storage for posts, scores, and metadata. Single-file database at `.noisepan/noisepan.db`. Schema supports deduplication by content hash, scoring metadata, and source tracking.

### Schema

```sql
CREATE TABLE posts (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    source       TEXT NOT NULL,           -- "telegram", "rss", "reddit"
    channel      TEXT NOT NULL,           -- channel/feed/subreddit name
    external_id  TEXT NOT NULL,           -- source-specific ID (msg_id, guid, etc.)
    text         TEXT,                    -- full text (nullable if privacy.store_full_text=false)
    snippet      TEXT NOT NULL,           -- first 200 chars, always stored
    text_hash    TEXT NOT NULL,           -- SHA-256 of full text for dedup
    url          TEXT,                    -- link to original
    posted_at    DATETIME NOT NULL,       -- when the post was published
    fetched_at   DATETIME NOT NULL,       -- when we fetched it
    UNIQUE(source, channel, external_id)
);

CREATE TABLE scores (
    post_id      INTEGER PRIMARY KEY REFERENCES posts(id),
    score        INTEGER NOT NULL DEFAULT 0,
    labels       TEXT,                    -- JSON array of label strings
    tier         TEXT NOT NULL DEFAULT 'ignore',  -- "read_now", "skim", "ignore"
    scored_at    DATETIME NOT NULL,
    explanation  TEXT                     -- JSON: why this score
);

CREATE TABLE metadata (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
```

### Scope

| File | Change |
|------|--------|
| `internal/store/store.go` | New file: `Store` struct with Open, Close, InsertPost, GetUnscored, SaveScore, GetPosts, Deduplicate |
| `internal/store/schema.go` | New file: embedded SQL schema, auto-migration on Open |
| `internal/store/store_test.go` | New file: tests for all operations |

### Acceptance criteria

- [ ] `store.Open(path)` creates database and runs schema migration
- [ ] `InsertPost` deduplicates by `(source, channel, external_id)` — upsert, not error
- [ ] `GetUnscored` returns posts without scores
- [ ] `SaveScore` writes score + labels + tier + explanation
- [ ] `GetPosts(since, tier)` queries by time window and optional tier filter
- [ ] Content hash computed on insert for cross-source deduplication
- [ ] `make test && make lint` pass

### Notes

- Use `modernc.org/sqlite` (pure Go, no CGO required) or `mattn/go-sqlite3` (CGO). Prefer `modernc.org/sqlite` for easier cross-compilation.
- Embed schema SQL using `//go:embed`
- Auto-migrate: check `metadata` table for schema version, run migrations if needed

---

### WO-N02: Config and taste profile loading

**Status:** `[x]` done
**Priority:** high — all commands need config

### Summary

Load `config.yaml` and `taste.yaml` from the config directory (default `.noisepan/`). Validate required fields, provide sane defaults, resolve env var references for secrets.

### Scope

| File | Change |
|------|--------|
| `internal/config/config.go` | New file: config struct, Load function, env var resolution |
| `internal/config/taste.go` | New file: taste profile struct, Load function |
| `internal/config/config_test.go` | New file: tests for loading, defaults, validation |

### Acceptance criteria

- [ ] `config.Load(dir)` reads `config.yaml` from dir
- [ ] `config.LoadTaste(path)` reads taste profile
- [ ] Env var references (`api_key_env: OPENAI_API_KEY`) resolved at load time
- [ ] Missing optional fields get defaults (top_n=7, since=24h, mode=heuristic)
- [ ] Missing required fields (channels list) return clear error
- [ ] `make test && make lint` pass

### Notes

- Use `gopkg.in/yaml.v3` for parsing
- Config struct should be flat — no deep nesting
- Taste profile weights are `map[string]int` for simplicity

---

### WO-N03: Source interface and Telegram collector

**Status:** `[x]` done
**Priority:** high — first source, validates the pipeline

### Summary

Define the `Source` interface and implement Telegram channel reading. The Telegram collector uses a Python helper script (Telethon) that outputs JSONL, which the Go binary reads and inserts into the store.

### Design decision: hybrid Python+Go

Telegram API client libraries in Go are unreliable for user-account access (MTProto is complex). Telethon (Python) is battle-tested. The boundary is clean:

- Python script: authenticate, fetch messages, emit JSONL to stdout
- Go binary: read JSONL, insert into store

The Python script ships in `scripts/collector_telegram.py` and is invoked by the Go source via `exec.Command`.

### Scope

| File | Change |
|------|--------|
| `internal/source/source.go` | New file: `Source` interface, `Post` struct |
| `internal/source/telegram.go` | New file: Telegram source — invokes Python collector, parses JSONL |
| `internal/source/telegram_test.go` | New file: tests with mock JSONL input |
| `scripts/collector_telegram.py` | New file: Telethon-based collector, outputs JSONL |
| `scripts/requirements.txt` | New file: `telethon>=1.36` |

### Source interface

```go
type Post struct {
    Source     string    // "telegram"
    Channel   string    // channel name
    ExternalID string   // message ID
    Text      string    // message text
    URL       string    // link to original message
    PostedAt  time.Time // when published
}

type Source interface {
    Name() string
    Fetch(since time.Time) ([]Post, error)
}
```

### JSONL format (Python → Go)

```json
{"channel":"devops_ru","msg_id":"12345","date":"2026-02-16T12:01:00Z","text":"...","url":"https://t.me/devops_ru/12345"}
```

### Acceptance criteria

- [ ] `Source` interface defined with `Name()` and `Fetch(since)`
- [ ] Telegram source invokes Python script, parses JSONL output
- [ ] Python script authenticates with Telegram API, fetches messages from configured channels
- [ ] JSONL output includes channel, msg_id, date, text, url
- [ ] Test with mock JSONL (no live Telegram in tests)
- [ ] Graceful error if Python/Telethon not installed (`noisepan doctor` checks this)
- [ ] `make test && make lint` pass

### Notes

- Telegram session file stored in `.noisepan/session/` — first run is interactive (enter phone + code)
- After first auth, subsequent runs are automatic
- `noisepan doctor` should check: Python available, Telethon installed, session exists
- Rate limit: fetch max 100 messages per channel per pull (Telegram limit)

---

### WO-N04: Taste scoring engine

**Status:** `[x]` done
**Priority:** high — the core differentiator

### Summary

Score posts against the taste profile. Apply keyword weights, rule-based scoring, and label assignment. Produce a scored post with tier (read_now/skim/ignore) and explanation.

### Scope

| File | Change |
|------|--------|
| `internal/taste/scorer.go` | New file: `Score(post, profile) ScoredPost` |
| `internal/taste/scorer_test.go` | New file: table-driven tests for scoring |

### Scoring algorithm

1. Start with base score 0
2. For each `high_signal` keyword found in text: add weight
3. For each `low_signal` keyword found in text: add weight (negative)
4. Apply rules: if `contains_any` matches, add `score_add` and assign labels
5. Deduplicate labels
6. Assign tier based on thresholds: `score >= read_now` → "read_now", `score >= skim` → "skim", else "ignore"
7. Build explanation: list of matched keywords/rules with their score contributions

### ScoredPost

```go
type ScoredPost struct {
    Post        source.Post
    Score       int
    Labels      []string
    Tier        string // "read_now", "skim", "ignore"
    Explanation []ScoreContribution
}

type ScoreContribution struct {
    Reason string // "keyword: kubernetes" or "rule: contains cve"
    Points int
}
```

### Acceptance criteria

- [ ] Keywords matched case-insensitively
- [ ] Multiple keyword matches in one post accumulate
- [ ] Rules with `contains_any` fire when any keyword matches
- [ ] Labels deduplicated and sorted
- [ ] Tier assigned from thresholds
- [ ] Explanation captures every score contribution
- [ ] `make test && make lint` pass

### Notes

- Keyword matching: use `strings.Contains(strings.ToLower(text), keyword)` — simple, fast, deterministic
- No embeddings, no ML — pure keyword+rule scoring. Add semantic scoring as a future WO if needed.
- The taste profile is reloaded on each run (no caching between runs)

---

### WO-N05: Heuristic summarizer

**Status:** `[x]` done
**Priority:** medium — useful without LLM

### Summary

Summarize scored posts without calling an external API. Extract key sentences, links, CVE IDs, version strings, and error patterns. Produce 1-3 bullet points per post.

### Scope

| File | Change |
|------|--------|
| `internal/summarize/heuristic.go` | New file: rule-based summarizer |
| `internal/summarize/heuristic_test.go` | New file: tests |
| `internal/summarize/summarize.go` | New file: `Summarizer` interface |

### Summarizer interface

```go
type Summary struct {
    Bullets []string // 1-3 key points
    Links   []string // extracted URLs
    CVEs    []string // extracted CVE IDs
}

type Summarizer interface {
    Summarize(text string) Summary
}
```

### Heuristic rules

1. Extract URLs (regexp)
2. Extract CVE IDs (`CVE-\d{4}-\d{4,}`)
3. Extract version strings (`v?\d+\.\d+\.\d+`)
4. First sentence of the post (up to first period or newline, max 120 chars)
5. If post mentions "breaking change", "deprecated", "removed" — include that sentence
6. If post has > 3 URLs, note "N links included"

### Acceptance criteria

- [ ] Produces 1-3 bullets for any text input
- [ ] Extracts URLs, CVEs, version strings
- [ ] Never returns empty summary (at minimum: first sentence)
- [ ] Handles empty/short text gracefully
- [ ] `make test && make lint` pass

---

### WO-N06: Terminal digest formatter

**Status:** `[x]` done
**Priority:** high — the user-facing output

### Summary

Format scored and summarized posts into a terminal digest. Group by tier (Read Now / Skim / Ignore count). Show score, labels, and summary bullets. ANSI colors when TTY.

### Scope

| File | Change |
|------|--------|
| `internal/digest/terminal.go` | New file: terminal formatter |
| `internal/digest/terminal_test.go` | New file: tests for output formatting |
| `internal/digest/digest.go` | New file: `Formatter` interface |

### Acceptance criteria

- [ ] Output grouped: Read Now (with bullets), Skim (titles only), Ignore (count only)
- [ ] ANSI colors for TTY, plain text for pipes
- [ ] Shows: score, labels, summary bullets, source channel
- [ ] Header: "noisepan — N channels, M posts, time window"
- [ ] Footer: "Ignored: N posts (noise suppressed)"
- [ ] `make test && make lint` pass

---

### WO-N07: CLI commands (pull, digest, run, init, doctor)

**Status:** `[x]` done
**Priority:** high — wires everything together

### Summary

Implement all Cobra commands that compose the pipeline: `init` (create config dir), `pull` (fetch from sources), `digest` (score + summarize + print), `run` (pull + digest), `doctor` (health checks), `explain` (scoring breakdown).

### Scope

| File | Change |
|------|--------|
| `internal/cli/init.go` | New file: create .noisepan/ with example configs |
| `internal/cli/pull.go` | New file: fetch from all sources, insert into store |
| `internal/cli/digest.go` | New file: score unscored, summarize, format, print |
| `internal/cli/run.go` | New file: pull + digest |
| `internal/cli/doctor.go` | New file: check Python, Telethon, config, DB |
| `internal/cli/explain.go` | New file: show scoring breakdown for a post |
| `internal/cli/root.go` | Register all subcommands, add global flags |

### Acceptance criteria

- [ ] `noisepan init` creates `.noisepan/` with `config.yaml` and `taste.yaml` from embedded examples
- [ ] `noisepan pull` fetches from all configured sources
- [ ] `noisepan digest` scores, summarizes, prints
- [ ] `noisepan digest --since 48h` respects time window
- [ ] `noisepan run` does pull + digest
- [ ] `noisepan doctor` checks: config exists, DB writable, Python available, Telethon installed, Telegram session exists
- [ ] `noisepan explain <id>` prints score contributions for a specific post
- [ ] `--config DIR` flag works on all commands
- [ ] `make test && make lint` pass

---

## Phase 2: Quality and Sources

### WO-N08: LLM summarizer backend

**Status:** `[x]` done
**Priority:** medium — enhances summaries for complex posts

### Summary

Add optional LLM-backed summarization for "Read Now" posts. Only called when `summarize.mode: llm` is set in config. Sends post text to OpenAI (or compatible API) with a focused prompt.

### Scope

| File | Change |
|------|--------|
| `internal/summarize/llm.go` | New file: LLM summarizer implementation |
| `internal/summarize/llm_test.go` | New file: tests with mock HTTP |

### Acceptance criteria

- [ ] Only called for posts with tier "read_now" (don't waste tokens on noise)
- [ ] Prompt: "Summarize for senior DevOps engineer. Focus on: breaking changes, incidents, security, architectural shifts. Max 4 bullets."
- [ ] Respects `max_tokens_per_post` from config
- [ ] API key read from env var specified in config
- [ ] Graceful fallback to heuristic if API fails
- [ ] `make test && make lint` pass

---

### WO-N09: RSS/Atom source

**Status:** `[x]` done
**Priority:** medium — second source type

### Summary

Add RSS/Atom feed reader as a source. Parse standard RSS 2.0 and Atom feeds, extract title + content + link + date, emit as Posts.

### Scope

| File | Change |
|------|--------|
| `internal/source/rss.go` | New file: RSS source implementation |
| `internal/source/rss_test.go` | New file: tests with fixture XML |

### Acceptance criteria

- [ ] Reads RSS 2.0 and Atom feeds
- [ ] Extracts: title, content/description, link, pubDate
- [ ] Respects `since` parameter — only returns posts newer than threshold
- [ ] Handles feed errors gracefully (timeout, malformed XML)
- [ ] `make test && make lint` pass

### Notes

- Use `github.com/mmcdole/gofeed` for feed parsing — handles both RSS and Atom
- Strip HTML from content before scoring (use `html.UnescapeString` + regex or `bluemonday`)

---

### WO-N10: Reddit source

**Status:** `[x]` done
**Priority:** low — third source, nice to have

### Summary

Add Reddit as a source. Read posts from configured subreddits via Reddit's JSON API (no OAuth needed for public subreddits).

### Scope

| File | Change |
|------|--------|
| `internal/source/reddit.go` | New file: Reddit source implementation |
| `internal/source/reddit_test.go` | New file: tests with mock JSON |

### Acceptance criteria

- [ ] Fetches from `https://www.reddit.com/r/<subreddit>/new.json`
- [ ] Extracts: title, selftext, url, created_utc
- [ ] Respects `since` parameter
- [ ] Respects Reddit rate limits (1 req/sec with User-Agent)
- [ ] `make test && make lint` pass

---

### WO-N11: JSON and Markdown output formats

**Status:** `[x]` done
**Priority:** low — enables piping and sharing

### Summary

Add `--format json` and `--format markdown` output modes alongside the default terminal formatter.

### Scope

| File | Change |
|------|--------|
| `internal/digest/json.go` | New file: JSON output |
| `internal/digest/markdown.go` | New file: Markdown output |

### Acceptance criteria

- [ ] `noisepan digest --format json` outputs valid JSON to stdout
- [ ] `noisepan digest --format markdown` outputs Markdown suitable for sharing
- [ ] Both respect the same tier grouping as terminal
- [ ] `make test && make lint` pass

---

### WO-N12: Post deduplication across sources

**Status:** `[x]` done
**Priority:** medium — prevents duplicate signal when same content appears in multiple channels

### Summary

Detect and merge duplicate posts that appear across different channels or sources. Use content hash matching — if two posts from different channels have the same text hash, keep one and link the other as "also seen in".

### Scope

| File | Change |
|------|--------|
| `internal/store/store.go` | Add `FindDuplicates` and `MergeDuplicates` methods |
| `internal/store/store_test.go` | Add dedup tests |

### Acceptance criteria

- [ ] Same text in two channels → scored once, displayed once with "also in: channel2"
- [ ] Dedup runs after pull, before scoring
- [ ] Original source preserved (first seen wins)
- [ ] `make test && make lint` pass

---

## Phase 2 Execution Order

```
WO-N08 (LLM summarizer) ─────────→ standalone
WO-N09 (RSS source) ─────────────→ standalone
WO-N10 (Reddit source) ──────────→ standalone
WO-N11 (Output formats) ─────────→ standalone
WO-N12 (Dedup) ──────────────────→ standalone
```

No dependencies between Phase 2 WOs — all can be parallelized.

Critical path for MVP: WO-N01 → WO-N02 → WO-N03 → WO-N04 → WO-N06 → WO-N07 (sequential, each builds on the previous).
WO-N05 (heuristic summarizer) can be built in parallel with N04-N06.

---

### WO-N13: forge-plan local source

**Status:** `[x]` done
**Priority:** low — after Phase 1 ships
**Depends on:** WO-N07

### Summary

Add a "local" source type that runs `forge-plan.sh` and ingests its output as posts. Each suggested action becomes a post scored by the taste engine. Allows `noisepan digest` to show repo status alongside external signals.

### Scope

| File | Change |
|------|--------|
| `internal/source/forgeplan.go` | New file: local source — runs forge-plan.sh, parses output into Posts |
| `internal/source/forgeplan_test.go` | New file: tests with mock forge-plan output |

### Acceptance criteria

- [ ] Implements `Source` interface (`Name()` returns "forgeplan", `Fetch()` runs script)
- [ ] Each suggested action becomes one Post with action description as text
- [ ] Configurable script path in `config.yaml` (`sources.forgeplan.script`)
- [ ] Graceful error if script not found or not executable
- [ ] `make test && make lint` pass

### Notes

- forge-plan.sh lives at `/Users/pashah/dev/claude-skills/scripts/forge-plan.sh` — config should allow overriding path
- Parse the "Suggested actions" section only, ignore headers
- This is a local-only source — no network, no API keys

---

## Phase 3: Correctness and Usability

### WO-N14: Honor digest limits and data retention

**Status:** `[x]` done
**Priority:** high — config promises behavior that isn't delivered

### Summary

Two config fields are loaded but never used: `storage.retain_days` (old posts never pruned) and `digest.top_n` / `digest.include_skims` (digest shows all posts regardless of limits). Wire both up.

### Scope

| File | Change |
|------|--------|
| `internal/store/store.go` | Add `PruneOld(ctx, retainDays) (int64, error)` — DELETE posts older than N days + their scores and also_in |
| `internal/store/store_test.go` | Add tests for PruneOld |
| `internal/cli/pull.go` | Call `db.PruneOld()` after `Deduplicate()` |
| `internal/cli/digest.go` | Apply TopN and IncludeSkims limits before building DigestInput |

### Acceptance criteria

- [ ] `PruneOld` deletes posts (and associated scores/also_in) older than `retain_days`
- [ ] `pull` calls PruneOld and reports count if > 0
- [ ] Digest limits read_now to `top_n` items and skims to `include_skims` items (sorted by score desc)
- [ ] Ignored posts are counted but never included in output (already works)
- [ ] `make test && make lint` pass

---

### WO-N15: Privacy enforcement (redaction and full-text control)

**Status:** `[x]` done
**Priority:** high — config promises privacy features that aren't implemented

### Summary

`privacy.store_full_text: false` is ignored — full text is always stored. `privacy.redact.patterns` are loaded but never applied. Wire both up so privacy config actually controls behavior.

### Scope

| File | Change |
|------|--------|
| `internal/privacy/redact.go` | New file: `Apply(text string, patterns []string) string` — compile and apply regex replacements |
| `internal/privacy/redact_test.go` | New file: tests for redaction |
| `internal/cli/pull.go` | If `!cfg.Privacy.StoreFullText`, set `Text: ""` in PostInput; if redact enabled, apply patterns to text before insert |

### Acceptance criteria

- [ ] When `store_full_text: false`, posts are stored with empty Text (snippet still populated)
- [ ] When `redact.enabled: true`, matching patterns are replaced with `[REDACTED]` in text before storage
- [ ] Redaction applies before snippet extraction (so snippet is also redacted)
- [ ] `make test && make lint` pass

---

### WO-N16: Source and channel filtering for digest

**Status:** `[x]` done
**Priority:** medium — usability improvement

### Summary

`GetPosts` only filters by time and tier. Add `--source` and `--channel` flags to `digest` so users can scope output.

### Scope

| File | Change |
|------|--------|
| `internal/store/store.go` | Extend `GetPosts` with optional source and channel filter params |
| `internal/store/store_test.go` | Add filter tests |
| `internal/cli/digest.go` | Add `--source` and `--channel` flags, pass to GetPosts |

### Acceptance criteria

- [ ] `noisepan digest --source rss` shows only RSS posts
- [ ] `noisepan digest --channel devops` shows only posts from that channel
- [ ] Flags can be combined
- [ ] `make test && make lint` pass

---

### WO-N17: Integration tests

**Status:** `[x]` done
**Priority:** medium — confidence in the full pipeline

### Summary

Add end-to-end tests that exercise the full pull→score→digest pipeline with a temp database. Currently only unit tests exist per package.

### Scope

| File | Change |
|------|--------|
| `internal/cli/pipeline_test.go` | New file: integration test seeding a temp DB, running scoring, verifying digest output through all formatters |

### Acceptance criteria

- [x] Test inserts posts, scores them, runs digest, verifies terminal/JSON/markdown output
- [x] Uses temp dir for DB and config — no external dependencies
- [x] `make test && make lint` pass

---

### WO-N18: Watch mode for continuous operation

**Status:** `[x]` done
**Priority:** low — nice to have for power users

### Summary

Add `--every <duration>` flag to `noisepan run` for continuous pull+digest on a timer. Graceful shutdown on SIGINT/SIGTERM.

### Scope

| File | Change |
|------|--------|
| `internal/cli/run.go` | Add `--every` flag, `time.Ticker` loop, signal handling |

### Acceptance criteria

- [x] `noisepan run --every 30m` pulls and digests every 30 minutes
- [x] Graceful shutdown on Ctrl-C
- [x] First run is immediate, then waits for interval
- [x] `make test && make lint` pass

---

## Phase 4: Signal Verification

### WO-N19: Entropia verification for digest posts

**Status:** `[x]` done
**Priority:** medium — adds source credibility signal to high-value posts
**Depends on:** WO-N07

### Summary

Add `noisepan verify` command that runs `entropia scan` on URLs from read_now posts and displays the support index alongside the digest. This lets users see whether high-signal posts are backed by authoritative sources or have evidence gaps.

Entropia (`entropia scan <url> --json`) outputs a JSON report with a support index (0-100), confidence level, conflict flag, and evidence signals. The verify command shells out to the entropia binary, parses the JSON, and prints a verification summary for each read_now post.

### Design

`noisepan verify` reads scored posts from the DB (same query as digest), filters to read_now tier, extracts URLs, runs `entropia scan <url> --json` for each, and prints results inline.

Output format:
```
noisepan verify — 6 read_now posts, 5 with URLs

--- Verification ---

  [14] cybersecurity — Infosec exec sold zero-day exploit kits
      https://www.reddit.com/r/cybersecurity/comments/.../
      entropia: support 72/100, confidence high, no conflict

  [10] Kubernetes — Telescope open-source log viewer
      https://www.reddit.com/r/kubernetes/comments/.../
      entropia: support 45/100, confidence medium, ⚠ conflict detected

  [9] netsec — Prompt Injection Standardization
      https://www.reddit.com/r/netsec/comments/.../
      entropia: skipped (reddit.com not scannable)
```

### Scope

| File | Change |
|------|--------|
| `internal/cli/verify.go` | New file: verify command — reads read_now posts, runs entropia, prints results |
| `internal/cli/verify_test.go` | New file: tests with mock entropia output |
| `internal/cli/doctor.go` | Add entropia binary check (optional, warn if missing) |
| `internal/cli/root.go` | Register verify subcommand |

### Acceptance criteria

- [ ] `noisepan verify` scans read_now post URLs with `entropia scan <url> --json`
- [ ] Displays support index, confidence, conflict flag per post
- [ ] Skips posts without URLs (with note)
- [ ] Skips domains known to be unscannable (reddit.com, t.me) with reason
- [ ] `noisepan doctor` warns if entropia binary not found in PATH
- [ ] `--since` and `--config` flags work (same as digest)
- [ ] Timeout per scan: 30s
- [ ] Per-URL errors are non-fatal (warn and continue to next post)
- [ ] `make test && make lint` pass

### Notes

- Entropia scan JSON structure: `{"url":"...","score":{"index":72,"confidence":"high","conflict":false,"signals":[...]}}`
- Run scans sequentially — entropia is network-heavy, parallel scans would overwhelm sources
- Skip posts without URLs (forgeplan actions, some telegram posts)
- Skip known-unscannable domains: reddit.com (returns 403 to entropia), t.me (requires auth)
- Consider caching entropia results in the DB to avoid re-scanning on repeated verify calls
- Entropia binary is a separate install (`brew install ppiankov/tap/entropia` or `go install`)

---

## Phase 5: Intelligence and Usability

### WO-N20: Stats command — feed and scoring analytics

**Status:** `[x]` done
**Priority:** high — the tool looking at itself, helps users tune their setup

### Summary

Add `noisepan stats` command that analyzes stored posts and scores to show per-source and per-channel signal-to-noise ratios, top-performing feeds, and scoring distribution. Helps users identify noisy feeds to drop and high-signal feeds to prioritize.

### Design

Query the posts and scores tables grouped by source and channel. Compute totals, tier distribution, and signal ratio (read_now + skim as % of total). Display as a ranked table.

Output format:
```
noisepan stats — 30 days, 4217 posts from 28 channels

--- Signal-to-Noise by Channel ---

  Channel                     Posts   Read Now   Skim   Ignored   Signal
  CISA advisories               47       31       12        4      91%
  Krebs on Security             38       18       14        6      84%
  Kubernetes Blog               22        8       10        4      82%
  r/cybersecurity              412       24       53      335      19%
  r/LocalLLaMA                 389        8       31      350       10%

--- Scoring Distribution ---

  Read Now:    142  (3.4%)
  Skim:        389  (9.2%)
  Ignored:    3686  (87.4%)

--- Stale Channels (no posts in 7+ days) ---

  Tigera Blog — last post 2026-02-03
```

### Scope

| File | Change |
|------|--------|
| `internal/store/store.go` | Add `GetStats(ctx, since) (StatsResult, error)` — aggregate query by source/channel with tier counts |
| `internal/store/store_test.go` | Add stats query tests |
| `internal/cli/stats.go` | New file: stats command, formats and prints analytics table |
| `internal/cli/stats_test.go` | New file: tests for stats output |
| `internal/cli/root.go` | Register statsCmd |

### Acceptance criteria

- [ ] `noisepan stats` shows per-channel signal-to-noise ratio ranked by signal %
- [ ] Shows overall scoring distribution (read_now / skim / ignored counts and %)
- [ ] Shows stale channels (no posts in 7+ days)
- [ ] `--since` flag controls time window (default: 30d)
- [ ] `make test && make lint` pass

### Notes

- Signal ratio = (read_now + skim) / total * 100
- Stale detection: compare latest `posted_at` per channel against current time
- This is a read-only analytics query — no mutations
- Consider adding `--format json` support for piping to other tools

---

### WO-N21: Feed health checks in doctor

**Status:** `[x]` done
**Priority:** medium — practical early warning for broken feeds

### Summary

Extend `noisepan doctor` to check feed health: detect stale feeds (no posts in 7+ days), feeds that always score as ignored (wrong feed for taste profile), and feeds that return HTTP errors. Currently doctor only checks config and DB.

### Scope

| File | Change |
|------|--------|
| `internal/cli/doctor.go` | Add feed health checks: stale detection, error-rate check, all-ignored check |
| `internal/store/store.go` | Add `GetChannelLastSeen(ctx) (map[string]time.Time, error)` — latest posted_at per channel |
| `internal/store/store_test.go` | Add tests for GetChannelLastSeen |

### Acceptance criteria

- [ ] Doctor warns about channels with no posts in 7+ days ("stale: last seen N days ago")
- [ ] Doctor warns about channels where 100% of posts scored as ignored ("all noise: consider removing or adjusting taste profile")
- [ ] Doctor warns are non-fatal (info-level, don't fail the check)
- [ ] Requires at least one completed pull to have data (skip checks if DB is empty)
- [ ] `make test && make lint` pass

### Notes

- Only check feeds that are still in the current config (don't warn about removed feeds)
- Stale threshold should be configurable in the future but hardcode 7 days for now
- This reuses the stats query infrastructure from N20 if implemented first

---

### WO-N22: Trending detection — cross-source mention counting

**Status:** `[x]` done
**Priority:** medium — surfaces breaking stories that appear across multiple feeds
**Depends on:** WO-N20

### Summary

When the same topic appears in multiple feeds within a time window, that's a stronger signal than any individual post score. Add mention counting to the digest: if a keyword or URL appears in N+ sources, annotate it as trending.

### Design

After scoring, scan read_now and skim posts for overlapping keywords or shared URLs. Group posts that share high-signal keywords (from the taste profile) appearing in 3+ different channels within the digest window. Display a trending section at the top of the digest.

Output format:
```
--- Trending (appeared in 3+ sources) ---

  "CVE-2026-1234" — mentioned in 5 channels
    CISA, Krebs on Security, BleepingComputer, r/netsec, r/cybersecurity

  "Kubernetes v1.33" — mentioned in 3 channels
    Kubernetes Blog, CNCF, r/kubernetes
```

### Scope

| File | Change |
|------|--------|
| `internal/taste/trending.go` | New file: `FindTrending(posts []ScoredPost, profile Profile, minSources int) []Trend` |
| `internal/taste/trending_test.go` | New file: tests for trending detection |
| `internal/digest/terminal.go` | Add trending section before Read Now |
| `internal/digest/json.go` | Add trending to JSON output |
| `internal/digest/markdown.go` | Add trending to Markdown output |
| `internal/digest/digest.go` | Add `Trending []Trend` to DigestInput |
| `internal/cli/digest.go` | Call FindTrending, pass to formatter |

### Acceptance criteria

- [ ] Detects high-signal keywords appearing in 3+ different channels
- [ ] Trending section appears at the top of digest output (all formats)
- [ ] Only considers keywords from the taste profile (not arbitrary words)
- [ ] Deduplicates: if CVE-2026-1234 matches both "cve" and a rule, count once
- [ ] `minSources` default is 3
- [ ] `make test && make lint` pass

### Notes

- Start simple: match taste profile keywords across channels, group by keyword
- Do NOT use fuzzy matching or NLP — keep it deterministic
- Shared URLs are a strong grouping signal (same link posted in multiple channels)
- Consider the dedup table (post_also_in) as an additional signal

---

### WO-N23: OPML feed import

**Status:** `[x]` done
**Priority:** low — onboarding convenience

### Summary

Add `noisepan import <file.opml>` command that reads an OPML file (standard RSS reader export format) and adds the feeds to `config.yaml`. Saves users from manually editing YAML when migrating from another RSS reader.

### OPML format

```xml
<opml version="2.0">
  <body>
    <outline text="Tech" title="Tech">
      <outline type="rss" text="Krebs" xmlUrl="https://krebsonsecurity.com/feed/" />
      <outline type="rss" text="CISA" xmlUrl="https://www.cisa.gov/cybersecurity-advisories/all.xml" />
    </outline>
  </body>
</opml>
```

### Scope

| File | Change |
|------|--------|
| `internal/cli/import.go` | New file: import command — parse OPML XML, extract feed URLs, merge into config.yaml |
| `internal/cli/import_test.go` | New file: tests with sample OPML |
| `internal/cli/root.go` | Register importCmd |

### Acceptance criteria

- [ ] `noisepan import feeds.opml` reads OPML and adds new feed URLs to config.yaml
- [ ] Skips feeds already present in config (no duplicates)
- [ ] Reports: "Added N feeds, skipped M duplicates"
- [ ] Handles nested OPML outlines (folders)
- [ ] Preserves existing config.yaml content (only appends to rss.feeds list)
- [ ] Validates that added URLs look like feed URLs (http/https)
- [ ] `make test && make lint` pass

### Notes

- Use `encoding/xml` from stdlib — no external dependency needed
- OPML is simple: iterate `<outline>` elements, extract `xmlUrl` attribute
- Write back to config.yaml using yaml.v3 Marshal — must preserve existing structure
- Consider `--dry-run` flag that shows what would be added without modifying config

---

### WO-N24: Digest output routing (file and webhook)

**Status:** `[x]` done
**Priority:** low — enables automated delivery without external scripting

### Summary

Add `--output` flag to write digest to a file, and `--webhook` flag to POST digest JSON to a URL. This lets users route digests to Slack, Discord, email pipelines, or local files without shell redirection or wrapper scripts.

### Design

`--output path` writes the formatted digest to a file instead of stdout. `--webhook url` POSTs the JSON-formatted digest to the given URL (always uses JSON format for webhooks regardless of `--format` flag).

### Scope

| File | Change |
|------|--------|
| `internal/cli/digest.go` | Add `--output` flag: write to file instead of stdout |
| `internal/cli/digest.go` | Add `--webhook` flag: POST JSON digest to URL |
| `internal/cli/run.go` | Inherit `--output` and `--webhook` flags |

### Acceptance criteria

- [ ] `noisepan digest --output ~/digest.md --format markdown` writes to file
- [ ] `noisepan digest --webhook https://hooks.slack.com/...` POSTs JSON to URL
- [ ] Webhook always sends JSON regardless of `--format`
- [ ] Webhook includes `Content-Type: application/json` header
- [ ] Webhook timeout: 10s, non-fatal on failure (warn and continue)
- [ ] `--output` and `--webhook` can be combined (write file AND post)
- [ ] `--output` creates parent directories if needed
- [ ] `make test && make lint` pass

### Notes

- Webhook payload is the same JSON structure as `--format json` output
- For Slack, users will need a Slack incoming webhook URL — document in setup guide
- Keep it simple: plain HTTP POST, no retry, no queue
- `--output -` means stdout (default behavior, for explicitness)

---

## Execution Order (Phase 5)

```
WO-N20 (stats) ──────→ WO-N22 (trending) — trending reuses stats infrastructure
WO-N21 (doctor feeds) → standalone, can reuse N20 queries
WO-N23 (OPML import) ─→ standalone
WO-N24 (output routing) → standalone
```

N20 is the highest-leverage starting point — it informs all other improvements.

---

## Phase 6: Performance, Reliability, and Operational Intelligence

### WO-N25: Database indexes and schema v2 migration

**Status:** `[x]` done
**Priority:** high — every query full-scans without indexes

### Summary

Add indexes on `posted_at`, `text_hash`, `source/channel`, and `tier`. Bump schema version to 2 with idempotent `CREATE INDEX IF NOT EXISTS`.

### Scope

| File | Change |
|------|--------|
| `internal/store/schema.sql` | Add 4 `CREATE INDEX IF NOT EXISTS` statements |
| `internal/store/schema.go` | Bump `schemaVersion` to 2 |
| `internal/store/store_test.go` | Test indexes exist via `sqlite_master`, verify v2 migration |

---

### WO-N26: Parallel RSS fetching

**Status:** `[x]` done
**Priority:** high — 30 feeds × 30s timeout = 15 min worst case

### Summary

Replace sequential RSS loop with bounded goroutine pool (10 workers). Errors remain non-fatal per feed.

### Scope

| File | Change |
|------|--------|
| `internal/source/rss.go` | Worker pool with `sync.WaitGroup` + channels, `rssMaxWorkers = 10` |

---

### WO-N27: `noisepan rescore` command

**Status:** `[x]` done
**Priority:** medium — taste profile changes leave stale scores

### Summary

Delete all existing scores, re-score every post in the time window using the current taste profile.

### Scope

| File | Change |
|------|--------|
| `internal/store/store.go` | Add `DeleteAllScores(ctx) (int64, error)` |
| `internal/store/store_test.go` | Test DeleteAllScores |
| `internal/cli/rescore.go` | New command: loads config + taste, deletes scores, re-scores all posts |
| `internal/cli/rescore_test.go` | Test rescore output and tier recalculation |

---

### WO-N28: Stats data maturity and JSON output

**Status:** `[x]` done
**Priority:** medium — prevents over-pruning young feeds, enables scripted monitoring

### Summary

Add `FirstSeen` to channel stats, `--format json` flag, and maturity annotation for channels with < 30 days of data.

### Scope

| File | Change |
|------|--------|
| `internal/store/store.go` | Add `FirstSeen time.Time` to `ChannelStats`, add `MIN(p.posted_at)` to query |
| `internal/cli/stats.go` | Add `--format json` flag, maturity annotation "(Nd data)" for young channels |
| `internal/cli/stats_test.go` | Test JSON output, maturity annotation, young vs mature feeds |

---

### WO-N29: RSS retry with exponential backoff

**Status:** `[x]` done
**Priority:** medium — transient network errors kill fetches immediately

### Summary

Add `fetchWithRetry()` wrapping `fetchFeed()`. Max 3 attempts, backoff 1s/2s/4s. Retry on timeout, 429, 5xx. No retry on 4xx (except 429).

### Scope

| File | Change |
|------|--------|
| `internal/source/rss.go` | Add `fetchWithRetry()`, `isRetryableError()`, injectable sleep for tests |
| `internal/source/rss_test.go` | Test transient→success, permanent failure, all retries exhausted |

---

## Execution Order (Phase 6)

```
N25 (indexes) ─────→ N28 (stats maturity) — needs FirstSeen from indexed query
N26 (parallel RSS) → N29 (RSS retry) — retry wraps the fetch function
N27 (rescore) ─────→ standalone
```

N25, N26, N27 started in parallel. N28 after N25. N29 after N26.
