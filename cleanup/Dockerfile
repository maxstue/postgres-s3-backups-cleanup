FROM golang:1.27.3-alpine3.24 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /cleanup ./cmd/cleanup

FROM alpine:3.24.3
RUN apk add --no-cache ca-certificates
COPY --from=builder /cleanup /usr/local/bin/cleanup
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/cleanup"]
