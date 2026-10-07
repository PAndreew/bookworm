# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS build
WORKDIR /src

# Copy the manifests first so dependency download is cached independently of source.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/server ./cmd/server
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/ingest ./cmd/ingest

FROM alpine:3.21
# ca-certificates for outbound HTTPS to Gutenberg; tzdata because streaks and
# quotas resolve the user's IANA timezone at runtime.
RUN apk add --no-cache ca-certificates tzdata

# Run as a non-root user.
RUN adduser -D -u 10001 bookworm
USER bookworm

WORKDIR /app
COPY --from=build /src/seed ./seed
COPY --from=build /out/server /usr/local/bin/server
COPY --from=build /out/ingest /usr/local/bin/ingest

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/server"]
