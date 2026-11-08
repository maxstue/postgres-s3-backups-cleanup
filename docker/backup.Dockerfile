FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine3.24 AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/backup ./cmd/backup
COPY internal/backup ./internal/backup
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-w -s" -o /out/backup ./cmd/backup

FROM alpine:3.24.3
ARG PG_VERSION='18'
RUN apk add --update --no-cache ca-certificates postgresql${PG_VERSION}-client gzip --repository=https://dl-cdn.alpinelinux.org/alpine/edge/main
COPY --from=builder /out/backup /usr/local/bin/backup
COPY --chmod=755 docker/backup-entrypoint.sh /usr/local/bin/backup-entrypoint
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/backup-entrypoint"]
