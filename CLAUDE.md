# noisepan

Signal extractor for noisy information streams. Go CLI, SQLite storage, keyword+rule scoring.

## Build & Test

```bash
make build    # produces bin/noisepan
make test     # go test -race -cover ./...
make lint     # golangci-lint run ./...
```

## Architecture

- `cmd/noisepan/main.go` — entry point, delegates to `internal/cli`
- `internal/cli/` — Cobra commands
- `internal/source/` — source interface + per-source implementations (telegram, rss, reddit)
- `internal/store/` — SQLite: posts, scores, metadata
- `internal/taste/` — scoring engine: keyword weights, rules, labels, thresholds
- `internal/summarize/` — heuristic (default) and LLM summarizers
- `internal/digest/` — output formatters (terminal, json, markdown)
- `internal/config/` — config + taste profile loading from YAML

## Conventions

- Go 1.25+, CGO enabled (SQLite)
- LDFLAGS: `-X .../internal/cli.Version=$(VERSION_NUM)` — VERSION_NUM has no `v` prefix
- Sources implement `source.Source` interface: `Name() string`, `Fetch(since time.Time) ([]Post, error)`
- Posts scored by `taste.Score(post, profile) ScoredPost`
- All storage local — no cloud, no telemetry

## Work Orders

See `docs/work-orders.md` for pending WOs.
