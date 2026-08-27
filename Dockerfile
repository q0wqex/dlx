# ==============================================================================
# Build Stage: Compile Go binary with embedded frontend assets
# ==============================================================================
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy module definitions and source code
COPY go.mod ./
COPY . .

# Build fully static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/dlx .

# ==============================================================================
# Final Stage: Lightweight production runtime with yt-dlp standalone and ffmpeg
# ==============================================================================
FROM alpine:3.21

# Install runtime dependencies (ffmpeg, ca-certificates for HTTPS, curl for healthcheck & downloads)
RUN apk add --no-cache \
    ca-certificates \
    ffmpeg \
    tzdata \
    curl \
    libc6-compat

# Download official standalone yt-dlp binary based on target architecture
ARG TARGETARCH
RUN set -eux; \
    case "${TARGETARCH}" in \
      "amd64"|"") YTDLP_BIN="yt-dlp_linux" ;; \
      "arm64")    YTDLP_BIN="yt-dlp_linux_aarch64" ;; \
      *) echo "Unsupported architecture: ${TARGETARCH}" && exit 1 ;; \
    esac; \
    curl -fsSL "https://github.com/yt-dlp/yt-dlp/releases/latest/download/${YTDLP_BIN}" -o /usr/local/bin/yt-dlp; \
    chmod a+rx /usr/local/bin/yt-dlp; \
    /usr/local/bin/yt-dlp --version

# Copy compiled Go binary
COPY --from=builder /app/dlx /usr/local/bin/dlx

# Create temporary directory for downloads
RUN mkdir -p /tmp/dlx && chmod 777 /tmp/dlx

# Default environment configuration
ENV PORT=8080 \
    YTDLP_PATH=/usr/local/bin/yt-dlp \
    FFMPEG_PATH=/usr/bin/ffmpeg \
    MAX_CONCURRENT_DOWNLOADS=2 \
    MAX_FILE_SIZE=5G \
    DOWNLOAD_TIMEOUT=30m \
    TEMP_DIR=/tmp/dlx

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD curl -f http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["/usr/local/bin/dlx"]
