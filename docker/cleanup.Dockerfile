FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine3.24 AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/cleanup ./cmd/cleanup
COPY internal/cleanup ./internal/cleanup
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-w -s" -o /out/cleanup ./cmd/cleanup

FROM alpine:3.24.3
RUN apk add --no-cache ca-certificates
COPY --from=builder /out/cleanup /usr/local/bin/cleanup
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/cleanup"]
