# syntax=docker/dockerfile:1

# ---- build ----
FROM golang:1.26-alpine AS build

WORKDIR /src

# Download modules first so this layer is cached between code changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO off so the binaries run on a bare Alpine image without libc surprises.
RUN CGO_ENABLED=0 GOOS=linux go build -mod=mod -trimpath -ldflags="-s -w" -o /out/server ./cmd/server && \
    CGO_ENABLED=0 GOOS=linux go build -mod=mod -trimpath -ldflags="-s -w" -o /out/seed-cli ./cmd/seed

# ---- run ----
FROM alpine:3.20

# ca-certificates is required for HTTPS calls to Meta's Graph API.
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 app

WORKDIR /app

COPY --from=build /out/server /app/server
COPY --from=build /out/seed-cli /app/seed-cli
# Seed data lives at /app/seed so `./seed-cli` finds it via default flags.
COPY seed /app/seed

USER app

EXPOSE 8080

CMD ["/app/server"]
