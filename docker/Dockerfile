# Stage 1: Build Caddy with L4 + rate-limit modules
FROM golang:1.26-alpine AS caddy-builder
RUN apk add --no-cache git
COPY internal/caddybuild/build.sh /build-caddy.sh
RUN . /build-caddy.sh && build_caddy /usr/local/bin/caddy && verify_caddy /usr/local/bin/caddy

# Stage 2: Build Vue frontend
FROM node:22-alpine AS frontend-builder
WORKDIR /build/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
COPY internal/branding/ /build/internal/branding/
ARG NODE_OPTIONS=--max-old-space-size=512
RUN npm run build

# Stage 3: Build PosternProxy binary (embeds frontend dist)
FROM golang:1.26-alpine AS app-builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /build/frontend/dist ./internal/ui/dist
ARG VERSION=dev
RUN . internal/caddybuild/build.sh \
    && GOTOOLCHAIN="$CADDY_GO_TOOLCHAIN" CGO_ENABLED=0 go build -ldflags="-s -w -X main.Version=${VERSION}" -o /posternproxy ./cmd/posternproxy \
    && GOTOOLCHAIN="$CADDY_GO_TOOLCHAIN" CGO_ENABLED=0 go build -ldflags="-s -w -X main.Version=${VERSION}" -o /posternproxy-agent ./cmd/posternproxy-agent

# Stage 4: Runtime image
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates wget iptables \
    && rm -rf /var/lib/apt/lists/*
COPY --from=caddy-builder /usr/local/bin/caddy /usr/local/bin/caddy
COPY --from=app-builder /posternproxy /usr/local/bin/posternproxy
COPY --from=app-builder /posternproxy-agent /usr/local/bin/posternproxy-agent
ENV POSTERNPROXY_CADDY_BIN=/usr/local/bin/caddy
COPY docker/entrypoint.sh /entrypoint.sh
COPY docker/supervise.sh /usr/local/lib/posternproxy/supervise.sh
RUN chmod +x /entrypoint.sh /usr/local/lib/posternproxy/supervise.sh && mkdir -p /data/certs

VOLUME /data
EXPOSE 80 443 81 443/udp

ENTRYPOINT ["/entrypoint.sh"]
