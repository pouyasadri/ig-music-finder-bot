# Instagram Audio Telegram Bot

An open-access Telegram bot written in Go following Clean Architecture and Test-Driven Development (TDD).

## Features
1. **Reel Extraction**: Receives Instagram Reel links (`https://www.instagram.com/reel/...` or `/p/...`) and extracts audio snippets using `yt-dlp` and `ffmpeg`.
2. **Music Recognition**: Fingerprints and identifies audio using the **ACRCloud** API.
3. **High-Quality Full Track**: If recognized, searches YouTube and downloads the highest quality MP3 with embedded metadata.
4. **Fallback Handling**: If unrecognized or if YouTube download fails, seamlessly falls back to sending the original Reel audio snippet.
5. **Concurrency Control**: Bounded worker pool (max 2 parallel workers) to protect VPS resources and prevent rate limits.
6. **Strict Lifecycle Cleanup**: Every request creates an isolated temporary directory and guarantees cleanup via `defer os.RemoveAll(...)`.

## Architecture & Layout

```
.
├── cmd/
│   └── bot/
│       └── main.go
├── internal/
│   ├── domain/
│   │   ├── model.go
│   │   └── errors.go
│   ├── usecase/
│   │   ├── interfaces.go
│   │   ├── process_reel.go
│   │   └── process_reel_test.go
│   └── adapter/
│       ├── extractor/
│       │   └── ytdlp_extractor.go
│       ├── recognizer/
│       │   └── acrcloud_recognizer.go
│       ├── downloader/
│       │   └── youtube_downloader.go
│       └── telegram/
│           └── bot_handler.go
├── cookies.txt.example
├── Dockerfile
├── docker-compose.yml
├── go.mod
└── go.sum
```

## Running Unit Tests

```bash
go test -v ./internal/usecase/...
```

## Deployment with Docker Compose

1. Copy and configure environment variables in `.env`:
   ```env
   TELEGRAM_BOT_TOKEN=your_telegram_bot_token
   ACR_HOST=identify-eu-west-1.acrcloud.com
   ACR_KEY=your_acr_access_key
   ACR_SECRET=your_acr_access_secret
   ```

2. (Optional) Provide Instagram `cookies.txt` for authenticated downloads if needed:
   ```bash
   cp cookies.txt.example cookies.txt
   ```

3. Build and launch the container:
   ```bash
   docker compose up -d --build
   ```
