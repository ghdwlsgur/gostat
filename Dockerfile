FROM golang:alpine AS builder
WORKDIR /build

# tzdata is needed at runtime: the dashboard renders response dates in
# Asia/Seoul, and a scratch image ships no zoneinfo of its own.
RUN apk add --no-cache ca-certificates tzdata && update-ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build \
  -ldflags='-w -s -extldflags "-static" -X main.gostatVersion=docker' \
  -o /build/gostat .

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /build/gostat /usr/local/bin/gostat
ENTRYPOINT ["/usr/local/bin/gostat"]
