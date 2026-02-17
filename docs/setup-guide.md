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
noisepan init
```

Creates `~/.noisepan/config.yaml` and `~/.noisepan/taste.yaml` in the current directory's `.noisepan/`. For a home-directory config:

```bash
noisepan --config ~/.noisepan init
```

## Config directory

noisepan looks for `.noisepan/` relative to the **current working directory** by default. If your config is in `~/.noisepan/`, you must either:

- Run noisepan from your home directory, or
- Pass `--config ~/.noisepan` on every command

## Telegram source setup

### 1. Get API credentials

1. Go to https://my.telegram.org
2. Log in with your phone number
3. Click "API development tools" and create an app
4. Note your `api_id` (number) and `api_hash` (hex string)

### 2. Set environment variables

The config fields `api_id_env` and `api_hash_env` are **names of environment variables**, not the credentials themselves. This keeps secrets out of config files.

```fish
# fish shell (~/.config/fish/config.fish)
set -x TELEGRAM_API_ID 12345678
set -x TELEGRAM_API_HASH 0123456789abcdef0123456789abcdef
```

```bash
# bash/zsh (~/.bashrc or ~/.zshrc)
export TELEGRAM_API_ID=12345678
export TELEGRAM_API_HASH=0123456789abcdef0123456789abcdef
```

**Common mistake:** putting the actual API ID/hash directly in `api_id_env` / `api_hash_env` fields. These fields hold env var **names** (e.g. `TELEGRAM_API_ID`), not values.

### 3. Install the collector script

The Telegram source uses a Python helper script (`collector_telegram.py`). When installed via Homebrew, the script is not bundled — you need to download it:

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
noisepan --config ~/.noisepan doctor
noisepan --config ~/.noisepan pull
```

## Tuning the taste profile

The default taste profile (`taste.yaml`) is tuned for English DevOps/security content. If your channels post in other languages or cover different topics, posts will score 0 and land in "Ignored".

To see why a post scored low:

```bash
noisepan --config ~/.noisepan explain <post-id>
```

To fix, edit `~/.noisepan/taste.yaml`:

- Add keywords your channels actually use (in the correct language)
- Lower thresholds if most content is relevant: `skim: 0`, `read_now: 2`
- Add rules for compound signals specific to your feeds

## Troubleshooting

| Problem | Cause | Fix |
|---------|-------|-----|
| `open .noisepan/config.yaml: no such file or directory` | Config dir is relative to CWD | Use `--config ~/.noisepan` or run from `~` |
| `telethon is not installed` | System python3 lacks telethon | Set `python_path` to venv python in config.yaml |
| `can't open file '.../collector_telegram.py': No such file` | Script not found at expected path | Set `script` path in config.yaml |
| `EOFError: EOF when reading a line` | Telegram session not created | Run collector_telegram.py manually once (see step 6) |
| All posts show as "Ignored" | Taste keywords don't match content | Edit taste.yaml keywords or lower thresholds |
| `api_id` / `api_hash` empty at runtime | Put values in `_env` fields instead of env var names | Set `api_id_env: TELEGRAM_API_ID` and export the actual value as env var |
