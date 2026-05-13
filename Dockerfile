# syntax=docker/dockerfile:1

ARG GO_VERSION=1.26.1
ARG DEBIAN_VERSION=bookworm

FROM golang:${GO_VERSION}-${DEBIAN_VERSION} AS builder
WORKDIR /src

COPY go.mod ./

# Need to uncomment this when go.sum is present (now we have no dependencies):
#
# RUN --mount=type=cache,target=/go/pkg/mod \
#     go mod download
#
# Also need to add go.sum after go.mod to the COPY command above.

COPY cmd/ ./cmd/
COPY internal/ ./internal/

RUN --mount=type=cache,target=/root/.cache/go-build \
  CGO_ENABLED=0 GOFLAGS="-buildvcs=false" \
  go build -trimpath -ldflags="-s -w" -o /dist/grooming-studio-api ./cmd

FROM debian:${DEBIAN_VERSION}-slim AS runner

RUN --mount=type=cache,target=/var/cache/apt \
  --mount=type=cache,target=/var/lib/apt/lists \
  apt-get update && \
  apt-get install -y --no-install-recommends \
  ca-certificates \
  curl

RUN groupadd --gid 10001 app \
  && useradd --uid 10001 --gid 10001 -M app

COPY --from=builder /dist/grooming-studio-api /usr/local/bin/grooming-studio-api

USER app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/grooming-studio-api"]
