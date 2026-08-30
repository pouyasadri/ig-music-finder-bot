FROM golang:1.23-bookworm AS builder

WORKDIR /app
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

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/bin/bot ./cmd/bot

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    python3 \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Copy ACRCloud shared library
COPY --from=builder /usr/lib/libacrcloud_extr_tool.so /usr/lib/libacrcloud_extr_tool.so
RUN ldconfig

# Install latest standalone yt-dlp binary
RUN curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o /usr/local/bin/yt-dlp && \
    chmod a+rx /usr/local/bin/yt-dlp

WORKDIR /app
COPY --from=builder /app/bin/bot /app/bot

CMD ["/app/bot"]
