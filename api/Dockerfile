# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS build
WORKDIR /src

# Copy the manifests first so dependency download is cached independently of source.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/server ./cmd/server
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/ingest ./cmd/ingest
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/cards ./cmd/cards

FROM alpine:3.21
# ca-certificates for outbound HTTPS to Gutenberg; tzdata because streaks and
# quotas resolve the user's IANA timezone at runtime.
COPY requirements-sentences.txt /tmp/requirements-sentences.txt
RUN apk add --no-cache ca-certificates tzdata python3 py3-pip \
 && python3 -m venv /opt/sentences \
 && /opt/sentences/bin/pip install --no-cache-dir -r /tmp/requirements-sentences.txt
ENV PYSBD_PYTHON=/opt/sentences/bin/python

# Run as a non-root user.
RUN adduser -D -u 10001 bookworm
USER bookworm

WORKDIR /app
COPY --from=build /src/seed ./seed
COPY --from=build /out/server /usr/local/bin/server
COPY --from=build /out/ingest /usr/local/bin/ingest
COPY --from=build /out/cards /usr/local/bin/cards

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/server"]
