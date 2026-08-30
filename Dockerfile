# syntax=docker/dockerfile:1.4

# Stage 1: Dependency & Binary Builder
FROM golang:1.23-bookworm AS builder

WORKDIR /app

# Cache Go modules download
COPY go.mod go.sum ./
RUN go mod download

# Set up ACRCloud shared library for the target architecture
RUN ARCH=$(uname -m) && \
    ACR_DIR=$(find /go/pkg/mod/github.com/acrcloud/acrcloud_sdk_golang* -maxdepth 0) && \
    if [ "$ARCH" = "aarch64" ]; then \
        cp $ACR_DIR/linux/arm/aarch64/acrcloud/libacrcloud_extr_tool.so /usr/lib/libacrcloud_extr_tool.so && \
        cp $ACR_DIR/linux/arm/aarch64/acrcloud/libacrcloud_extr_tool.so $ACR_DIR/acrcloud/libacrcloud_extr_tool.so; \
    else \
        cp $ACR_DIR/linux/x86-64/acrcloud/libacrcloud_extr_tool.so /usr/lib/libacrcloud_extr_tool.so && \
        cp $ACR_DIR/linux/x86-64/acrcloud/libacrcloud_extr_tool.so $ACR_DIR/acrcloud/libacrcloud_extr_tool.so; \
    fi

# Build binary with Go build cache and stripped debugging symbols
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o /app/bin/bot ./cmd/bot

# Stage 2: Download yt-dlp binary in parallel
FROM curlimages/curl:latest AS downloader
RUN curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o /tmp/yt-dlp && \
    chmod a+rx /tmp/yt-dlp

# Stage 3: Minimal Runtime
FROM debian:bookworm-slim

# Install runtime packages with apt cache mount & pip cache
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    python3 \
    python3-pip \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Install python dependencies with pip cache
RUN --mount=type=cache,target=/root/.cache/pip \
    pip3 install --no-cache-dir --break-system-packages shazamio

# Copy ACRCloud shared library
COPY --from=builder /usr/lib/libacrcloud_extr_tool.so /usr/lib/libacrcloud_extr_tool.so
RUN ldconfig

# Copy standalone yt-dlp binary from downloader stage
COPY --from=downloader /tmp/yt-dlp /usr/local/bin/yt-dlp

WORKDIR /app
COPY --from=builder /app/bin/bot /app/bot
COPY scripts/ /app/scripts/

CMD ["/app/bot"]
