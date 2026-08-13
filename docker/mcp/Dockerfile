# Production image for the Remote MCP (Streamable HTTP) transport.
# Mirrors docker/cloud/Dockerfile's build pattern; runs `engram mcp --transport=http`
# as the container's primary process instead of `engram cloud serve`.

FROM --platform=$BUILDPLATFORM golang:1.25.10-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# modernc.org/sqlite is pure-Go (no CGO needed), consistent with docker/cloud/Dockerfile.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -o /out/engram ./cmd/engram

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata curl
RUN adduser -D -u 10001 engram

# Persistent data directory. ENGRAM_DATA_DIR defaults to $HOME/.engram (see
# internal/store/store.go); HOME is set to /home/engram below, so the default
# resolves to /home/engram/.engram — this is the path the Coolify volume must
# mount. Pre-create it with correct ownership so the non-root user can write.
RUN mkdir -p /home/engram/.engram && chown -R engram:engram /home/engram

USER engram
WORKDIR /home/engram
ENV HOME=/home/engram

COPY --from=builder /out/engram /usr/local/bin/engram

# Configurable via ENGRAM_HTTP_ADDR / ENGRAM_MCP_PORT (see cmd/engram/http_transport.go);
# 8080 is the documented default (docs/REMOTE-MCP.md).
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -fsS http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["engram"]
CMD ["mcp", "--transport=http"]
