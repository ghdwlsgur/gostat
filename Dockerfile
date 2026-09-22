# syntax=docker/dockerfile:1

ARG GO_VERSION=1.26

# The builder runs on the machine's own architecture and cross-compiles for the
# target, so a multi-arch build needs no emulation.
FROM --platform=${BUILDPLATFORM} golang:${GO_VERSION}-alpine AS builder

# tzdata because the dashboard renders response dates in Asia/Seoul and a
# scratch image carries no zoneinfo of its own. ca-certificates is not read
# while certificate verification stays off, but an http client image without a
# trust store is a trap for whoever turns verification on.
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X main.gostatVersion=${VERSION}" -o /out/gostat .

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /out/gostat /usr/local/bin/gostat

# scratch has no /etc/passwd, so an unprivileged user can only be named by id.
USER 65532:65532

ENTRYPOINT ["/usr/local/bin/gostat"]
