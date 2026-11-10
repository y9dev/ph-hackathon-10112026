# syntax=docker/dockerfile:1
# Stage 1: build the SPA into backend/internal/webui/dist
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
# vite outDir points to ../backend/internal/webui/dist
RUN mkdir -p /src/backend/internal/webui && npm run build

# Stage 2: build the Go binary with the embedded SPA
FROM golang:1.26-alpine AS api
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=web /src/backend/internal/webui/dist ./internal/webui/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/modus-voice ./cmd/server

# Stage 3: minimal runtime
FROM alpine:3.20
RUN adduser -D -u 10001 app && mkdir -p /data && chown app /data
USER app
ENV ADDR=:8080 DATA_DIR=/data DEMO_MODE=false SEED_DEMO=false
EXPOSE 8080
VOLUME ["/data"]
COPY --from=api /out/modus-voice /usr/local/bin/modus-voice
ENTRYPOINT ["modus-voice"]
