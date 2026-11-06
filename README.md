# Instagram / SoundCloud Audio Telegram Bot

An open-access Telegram bot written in Go following Clean Architecture.

## Features

1. **Reel Extraction**: Receives Instagram Reel/post links (`/reel/`, `/p/`, `/share/reel/`) or uploaded audio/video and extracts a recognition snippet with `yt-dlp` + `ffmpeg`.
2. **Multi-Engine Music Recognition**: Fingerprints audio via **ShazamIO → AudD → ACRCloud**, with Instagram metadata scrape as a final fallback. Consensus scoring picks the best match.
3. **High-Quality Full Track**: When recognized, searches SoundCloud then YouTube and downloads the best available MP3 with embedded metadata.
4. **Validated Provider Fallback**: Rejects short previews or duration-mismatched files and falls back to the original Reel audio when no complete track is available.
5. **SoundCloud Links**: Direct SoundCloud track URLs are downloaded and delivered without the recognition pipeline.
6. **Concurrency Control**: Bounded worker pool + admission queue, per-user rate limits, and per-URL leases to protect VPS resources.
7. **Strict Lifecycle Cleanup**: Every request creates an isolated temporary directory and guarantees cleanup via `defer os.RemoveAll(...)`.

## Architecture & Layout

```
.
├── cmd/bot/main.go                 # load config, signals, app.Start
├── internal/
│   ├── app/                        # composition root (wiring)
│   ├── domain/                     # entities + ports (DIP boundary)
│   ├── usecase/                    # ProcessMedia orchestration
│   ├── adapter/
│   │   ├── extractor/              # yt-dlp reel extraction
│   │   ├── recognizer/             # Shazam / AudD / ACR / scrape + fallback
│   │   ├── downloader/             # SoundCloud / YouTube + validation
│   │   └── telegram/               # Telegram bot adapter
│   ├── storage/sqlite/             # SQLite implementations of domain ports
│   └── config/
├── scripts/shazam_recognize.py
├── cookies.txt.example
├── Makefile
├── Dockerfile
├── docker-compose.yml
├── go.mod
└── go.sum
```

Dependency rule: `cmd → app → adapter/storage → usecase → domain`.

## Makefile

Run `make help` to list targets. Common ones:

| Target | Description |
|--------|-------------|
| `make build` | Build `bin/bot` |
| `make run` | Build and run locally (loads `.env` if present) |
| `make test` | Run all unit tests |
| `make test-race` | Run tests with the race detector |
| `make fmt` / `make vet` | Format / static analysis |
| `make tidy` | `go mod tidy` + verify |
| `make cookies` | Create `cookies.txt` from the example if missing |
| `make docker-up` | Build and start with Docker Compose |
| `make docker-down` | Stop containers |
| `make docker-logs` | Follow container logs |
| `make clean` | Remove build artifacts |

## Local development

Prerequisites: Go 1.27+, `ffmpeg` / `ffprobe`, `yt-dlp`, and (for Shazam) Python 3 with `shazamio`.

1. Configure `.env` (see variables below).
2. Optionally set up Instagram cookies: `make cookies`
3. Build and run:

```bash
make run
```

Or build only:

```bash
make build   # → bin/bot
```

## Running Unit Tests

```bash
make test
# or
go test ./...
```

## Deployment with Docker Compose

1. Copy and configure environment variables in `.env`:
   ```env
   TELEGRAM_BOT_TOKEN=your_telegram_bot_token
   # At least one recognizer is required.
   # Shazam defaults to /app/scripts/shazam_recognize.py when SHAZAM_SCRIPT_PATH is unset.
   # Set SHAZAM_SCRIPT_PATH= (empty) to disable Shazam; then ACR and/or AudD are required.
   # ACR_HOST=identify-eu-west-1.acrcloud.com
   # ACR_KEY=your_acr_access_key
   # ACR_SECRET=your_acr_access_secret
   # AUDD_API_TOKEN=
   # SHAZAM_SCRIPT_PATH=/app/scripts/shazam_recognize.py
   # Optional
   CHANNEL_ID=
   COOKIES_PATH=/app/cookies.txt
   DATABASE_PATH=/app/data/bot.db
   WORKER_COUNT=5
   QUEUE_LIMIT=20
   RATE_LIMIT=10
   RATE_WINDOW=1h
   REQUEST_TIMEOUT=3m
   LEASE_TTL=5m
   CACHE_TTL=24h
   MAX_UPLOAD_BYTES=51380224
   EXTRACT_TIMEOUT=60s
   YOUTUBE_DOWNLOAD_TIMEOUT=90s
   SOUNDCLOUD_DOWNLOAD_TIMEOUT=45s
   RECOGNIZE_SHAZAM_TIMEOUT=4s
   RECOGNIZE_AUDD_TIMEOUT=3s
   RECOGNIZE_ACR_TIMEOUT=3s
   RECOGNIZE_SCRAPE_TIMEOUT=3s
   AUDD_HTTP_TIMEOUT=5s
   ```

   Recognizers are wired only when configured. ACR credentials must be set as a complete trio when used. AudD is optional. Shazam is on by default in Docker; clear `SHAZAM_SCRIPT_PATH` to opt out.

2. (Optional) Provide Instagram `cookies.txt` for authenticated downloads:
   ```bash
   make cookies
   # or: cp cookies.txt.example cookies.txt
   ```

3. Build and launch:
   ```bash
   make docker-up
   # or: docker compose up -d --build
   ```

The bot persists its SQLite database in the `bot-data` Docker volume (cache, leases, rate limits, users/history/favorites). Temporary media lives on memory-backed `/tmp` and is removed after each request. Requests over the configured per-user rate limit or global queue limit are rejected with a retry message.

Downloaded tracks are accepted only when `ffprobe` can read their duration and that duration is compatible with the recognized track. Provider attempts use isolated temporary directories, so a rejected SoundCloud preview cannot be mistaken for a successful fallback download.

## User commands

- `/start` or `/help` — show supported inputs and commands.
- `/history` — view recent processed links and media.
- `/favorites` — view saved recognized tracks.
- `/settings` — choose default output mode: full track, original audio, or both.
- `/forget` — delete the current user's stored profile and request history.

Supported inputs:

- Public Instagram Reel/post links
- SoundCloud track links
- Uploaded audio/video files (processed through the recognition pipeline)

## Telegram UX

Processing and error messages are localized in Persian; technical details stay in structured JSON logs. History and favorites are available via commands; settings use inline controls for output mode. Callbacks for favorites and channel forwarding are user-triggered from delivered tracks.
