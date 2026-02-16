#!/usr/bin/env python3
"""Telegram channel collector for noisepan.

Fetches messages from Telegram channels using Telethon and emits JSONL
to stdout. Errors go to stderr. Exit code 0 on success, 1 on failure.
"""

import argparse
import asyncio
import json
import sys
from datetime import datetime, timezone
from pathlib import Path

try:
    from telethon import TelegramClient
    from telethon.errors import ChannelPrivateError, FloodWaitError
except ImportError:
    print("telethon is not installed. Run: pip install telethon", file=sys.stderr)
    sys.exit(1)

MAX_MESSAGES_PER_CHANNEL = 100


def parse_args():
    parser = argparse.ArgumentParser(description="Fetch Telegram channel messages")
    parser.add_argument("--api-id", required=True, type=int)
    parser.add_argument("--api-hash", required=True)
    parser.add_argument("--session-dir", required=True)
    parser.add_argument("--channels", required=True, help="Comma-separated channel names")
    parser.add_argument("--since", required=True, help="ISO8601 timestamp")
    return parser.parse_args()


async def fetch_channel(client, channel_name, since):
    """Fetch messages from a single channel, yield dicts."""
    clean_name = channel_name.lstrip("@")
    try:
        entity = await client.get_entity(channel_name)
    except ChannelPrivateError:
        print(f"channel {channel_name}: private or not joined", file=sys.stderr)
        return
    except Exception as e:
        print(f"channel {channel_name}: {e}", file=sys.stderr)
        return

    async for message in client.iter_messages(
        entity, limit=MAX_MESSAGES_PER_CHANNEL
    ):
        if message.date.replace(tzinfo=timezone.utc) < since:
            break
        if not message.text:
            continue
        yield {
            "channel": clean_name,
            "msg_id": str(message.id),
            "date": message.date.replace(tzinfo=timezone.utc).isoformat(),
            "text": message.text,
            "url": f"https://t.me/{clean_name}/{message.id}",
        }


async def main():
    args = parse_args()

    since = datetime.fromisoformat(args.since).replace(tzinfo=timezone.utc)
    channels = [c.strip() for c in args.channels.split(",") if c.strip()]

    session_dir = Path(args.session_dir)
    session_dir.mkdir(parents=True, exist_ok=True)
    session_path = str(session_dir / "noisepan")

    client = TelegramClient(session_path, args.api_id, args.api_hash)

    try:
        await client.start()

        for channel in channels:
            try:
                async for msg_dict in fetch_channel(client, channel, since):
                    line = json.dumps(msg_dict, ensure_ascii=False)
                    print(line, flush=True)
            except FloodWaitError as e:
                print(
                    f"rate limited for {e.seconds}s on {channel}, skipping",
                    file=sys.stderr,
                )
                continue
    finally:
        await client.disconnect()


if __name__ == "__main__":
    asyncio.run(main())
