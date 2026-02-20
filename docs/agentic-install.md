# Agentic Installation Pattern

How an autonomous agent discovers, installs, configures, and composes CLI tools without plugins.

## Observed behavior

An autonomous agent (OpenClaw) was given a general mission: "set up daily security intelligence on a server." Without any plugin framework, SDK, or registry, the agent:

1. **Discovered tools** from conversation context — the operator mentioned noisepan and entropia
2. **Read the READMEs** to understand what each tool does
3. **Installed via Homebrew**: `brew install ppiankov/tap/noisepan ppiankov/tap/entropia`
4. **Initialized config**: `noisepan --config ~/.noisepan init`
5. **Edited config files** — added 40 RSS feeds covering CISA, Krebs, Cloudflare, Kubernetes, etc.
6. **Tuned taste profile** — set keyword weights for CVE, zero-day, kubernetes, supply chain
7. **Composed a pipeline**: `noisepan pull && noisepan digest && noisepan verify`
8. **Set up cron**: daily 6 AM digest with entropia verification, output to `/tmp/digest.json`
9. **Verified the setup**: ran `noisepan doctor` and `noisepan run` to confirm everything works

No plugin was installed. No SDK was imported. No registry was queried. The agent used the same interface a human would: Homebrew + CLI + config files.

## Why this works

The tools follow the ANCC (Agent-Native CLI Convention):

- **Single binary** — `brew install` gives you everything
- **Deterministic** — same config, same feeds, same scores every time
- **Structured output** — `--format json` for machine consumption
- **Bounded jobs** — each invocation does one thing and exits
- **SKILL.md** — agent-readable documentation at repo root
- **Init command** — scaffolds config with sensible defaults

## What makes tools agent-friendly

1. **Homebrew formula** — agents know how to `brew install`
2. **`init` command** — agents can scaffold config without guessing file locations
3. **Config files, not flags** — complex configuration goes in YAML, not 30 flags
4. **`--format json`** — agents can parse output programmatically
5. **`doctor` command** — agents can verify their own setup
6. **Clear error messages** — stderr explains what went wrong, exit code 1 on failure
7. **SKILL.md** — purpose, commands, flags, JSON output structure, parsing examples

## What agents don't need

- Plugin frameworks (MCP, OpenAPI, custom SDKs)
- Plugin registries or marketplaces
- API keys for the tool itself (only for external services like LLM providers)
- Interactive installation wizards
- GUI configuration
- Runtime dependencies beyond the binary itself

## Replicating this pattern

For your own tools:

```bash
# 1. Ensure your tool is installable via package manager
brew install your-org/tap/your-tool

# 2. Add an init command
your-tool init

# 3. Add structured output
your-tool run --format json

# 4. Add a health check
your-tool doctor

# 5. Create SKILL.md at repo root
# See noisepan/SKILL.md or entropia/SKILL.md for examples

# 6. Link from README
# Add "Agent Integration" section pointing to SKILL.md
```
