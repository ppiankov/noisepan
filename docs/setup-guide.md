# noisepan Setup Guide

## Install

```bash
brew install ppiankov/tap/noisepan
```

Or build from source:

```bash
make build   # produces bin/noisepan
```

## Initialize

```bash
noisepan --config ~/.noisepan init
```

Creates `~/.noisepan/config.yaml` and `~/.noisepan/taste.yaml`.

## Shell setup

noisepan looks for `.noisepan/` relative to the **current working directory** by default. To avoid passing `--config` every time, add a shell alias.

### fish (~/.config/fish/config.fish)

```fish
# Telegram API credentials
set -x TELEGRAM_API_ID 12345678
set -x TELEGRAM_API_HASH 0123456789abcdef0123456789abcdef

# Alias so noisepan always finds config
alias noisepan "noisepan --config ~/.noisepan"
```

Apply: `source ~/.config/fish/config.fish`

### bash (~/.bashrc)

```bash
# Telegram API credentials
export TELEGRAM_API_ID=12345678
export TELEGRAM_API_HASH=0123456789abcdef0123456789abcdef

# Alias so noisepan always finds config
alias noisepan="noisepan --config ~/.noisepan"
```

Apply: `source ~/.bashrc`

### zsh (~/.zshrc)

```zsh
# Telegram API credentials
export TELEGRAM_API_ID=12345678
export TELEGRAM_API_HASH=0123456789abcdef0123456789abcdef

# Alias so noisepan always finds config
alias noisepan="noisepan --config ~/.noisepan"
```

Apply: `source ~/.zshrc`

After setting up the alias, all commands work without `--config`:

```bash
noisepan pull
noisepan digest
noisepan run
noisepan doctor
```

## Daily usage

```bash
# Pull posts from all sources, then show digest
noisepan run

# Or step by step
noisepan pull          # fetch from all sources into SQLite
noisepan digest        # score, summarize, display

# Filter by source or channel
noisepan digest --source rss
noisepan digest --channel kubernetes

# Different time window
noisepan digest --since 48h

# Output formats
noisepan digest --format json
noisepan digest --format markdown > digest.md

# Continuous mode (re-runs every 30 minutes, Ctrl+C to stop)
noisepan run --every 30m

# Debug a post's score
noisepan explain 42

# Health check
noisepan doctor
```

## Cron / scheduled runs

### cron (bash/zsh)

```bash
# crontab -e
0 8 * * * TELEGRAM_API_ID=12345678 TELEGRAM_API_HASH=abc123 /opt/homebrew/bin/noisepan --config /Users/you/.noisepan run --format md > /tmp/noisepan-digest.md 2>&1
```

### fish (using crontab)

Fish doesn't run in cron. Use the full binary path and pass env vars inline as shown above.

### launchd (macOS)

Create `~/Library/LaunchAgents/com.noisepan.daily.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.noisepan.daily</string>
    <key>ProgramArguments</key>
    <array>
        <string>/opt/homebrew/bin/noisepan</string>
        <string>--config</string>
        <string>/Users/you/.noisepan</string>
        <string>run</string>
        <string>--format</string>
        <string>md</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>TELEGRAM_API_ID</key>
        <string>12345678</string>
        <key>TELEGRAM_API_HASH</key>
        <string>your_hash_here</string>
    </dict>
    <key>StartCalendarInterval</key>
    <dict>
        <key>Hour</key>
        <integer>8</integer>
        <key>Minute</key>
        <integer>0</integer>
    </dict>
    <key>StandardOutPath</key>
    <string>/tmp/noisepan-digest.md</string>
    <key>StandardErrorPath</key>
    <string>/tmp/noisepan-error.log</string>
</dict>
</plist>
```

Load: `launchctl load ~/Library/LaunchAgents/com.noisepan.daily.plist`

## Telegram source setup

### 1. Get API credentials

1. Go to https://my.telegram.org
2. Log in with your phone number
3. Click "API development tools" and create an app
4. Note your `api_id` (number) and `api_hash` (hex string)

### 2. Set environment variables

The config fields `api_id_env` and `api_hash_env` are **names of environment variables**, not the credentials themselves. This keeps secrets out of config files that might get committed to git.

See the shell setup section above for how to export them in your shell.

**Common mistake:** putting the actual API ID/hash directly in `api_id_env` / `api_hash_env` fields. These fields hold env var **names** (e.g. `TELEGRAM_API_ID`), not values.

### 3. Install the collector script

The Telegram source uses a Python helper script (`collector_telegram.py`). When installed via Homebrew, the script is not bundled — download it:

```bash
mkdir -p ~/scripts
curl -o ~/scripts/collector_telegram.py \
  https://raw.githubusercontent.com/ppiankov/noisepan/main/scripts/collector_telegram.py
```

### 4. Set up a Python venv

The collector requires the `telethon` library. Use a venv to avoid polluting the system Python:

```bash
python3 -m venv ~/.noisepan/venv
~/.noisepan/venv/bin/pip install telethon
```

### 5. Configure paths in config.yaml

```yaml
sources:
  telegram:
    api_id_env: TELEGRAM_API_ID
    api_hash_env: TELEGRAM_API_HASH
    session_dir: /Users/you/.noisepan/session
    script: /Users/you/scripts/collector_telegram.py
    python_path: /Users/you/.noisepan/venv/bin/python
    channels:
      - "@channel_name"
```

Use **absolute paths** for `session_dir`, `script`, and `python_path`. Tilde (`~`) is not expanded by Go.

### 6. Create a Telegram session (one-time)

The first run requires interactive login. Run the collector manually:

```bash
~/.noisepan/venv/bin/python ~/scripts/collector_telegram.py \
  --api-id "$TELEGRAM_API_ID" \
  --api-hash "$TELEGRAM_API_HASH" \
  --session-dir ~/.noisepan/session \
  --channels "@your_channel" \
  --since 2026-01-01T00:00:00Z
```

Telethon will prompt for your phone number and a verification code sent via Telegram. After authenticating, a session file is saved in the session directory. Subsequent runs are automatic.

### 7. Verify

```bash
noisepan doctor
noisepan pull
```

## Reddit via RSS

Reddit's public JSON API requires pre-approved OAuth since November 2025. Use their RSS feeds instead:

```yaml
rss:
  feeds:
    - "https://www.reddit.com/r/devops/.rss"
    - "https://www.reddit.com/r/kubernetes/.rss"
    - "https://www.reddit.com/r/netsec/.rss"
    - "https://www.reddit.com/r/cybersecurity/.rss"
    - "https://www.reddit.com/r/devsecops/.rss"
    - "https://www.reddit.com/r/LocalLLaMA/.rss"
reddit:
  subreddits: []    # leave empty, use RSS instead
```

## Tuning the taste profile

The default taste profile (`taste.yaml`) is tuned for English DevOps/security content. If your channels post in other languages or cover different topics, posts will score 0 and land in "Ignored".

To see why a post scored low:

```bash
noisepan explain <post-id>
```

To fix, edit `~/.noisepan/taste.yaml`:

- Add keywords your channels actually use (in the correct language)
- Lower thresholds if most content is relevant: `skim: 0`, `read_now: 2`
- Add rules for compound signals specific to your feeds

## Troubleshooting

| Problem | Cause | Fix |
|---------|-------|-----|
| `open .noisepan/config.yaml: no such file or directory` | Config dir is relative to CWD | Use `--config ~/.noisepan` or set up the shell alias |
| `telethon is not installed` | System python3 lacks telethon | Set `python_path` to venv python in config.yaml |
| `can't open file '.../collector_telegram.py': No such file` | Script not found at expected path | Set `script` path in config.yaml |
| `EOFError: EOF when reading a line` | Telegram session not created yet | Run collector_telegram.py manually once (see step 6) |
| `invalid int value: ''` for api-id | Env vars not set in current shell | Export `TELEGRAM_API_ID` and `TELEGRAM_API_HASH` (see shell setup) |
| All posts show as "Ignored" | Taste keywords don't match content | Edit taste.yaml keywords or lower thresholds |
| `api_id` / `api_hash` empty at runtime | Put values in `_env` fields instead of env var names | Set `api_id_env: TELEGRAM_API_ID` and export the actual value as env var |
| Reddit returns 403 | Reddit killed public JSON API | Use RSS feeds instead (see Reddit via RSS section) |
