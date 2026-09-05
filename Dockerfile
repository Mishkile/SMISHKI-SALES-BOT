# syntax=docker/dockerfile:1

# --- Base Stage ---
# Dependencies are downloaded once and cached across stages.
FROM golang:1.27-alpine AS base
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# --- Development Stage ---
# Source is bind-mounted by docker-compose; `go run .` recompiles on restart.
FROM base AS development
COPY . .
CMD ["go", "run", "."]

# --- Build Stage ---
FROM base AS build
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/jsts-salebot .

# --- Production Stage ---
# Locales are embedded in the binary. config.json is NOT baked in: mount it
# (writable, so /config edits persist) at /app/config.json.
FROM alpine:3.22 AS production
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 bot
WORKDIR /app
COPY --from=build /out/jsts-salebot /app/jsts-salebot

# Security: run as a non-privileged user
USER bot

CMD ["/app/jsts-salebot"]
