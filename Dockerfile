# Build stage
FROM golang:1.24-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o /build/server \
    ./cmd/server

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary and static files
COPY --from=builder /build/server /app/server
COPY --from=builder /build/web/static /app/web/static

EXPOSE 8080 9000

ENTRYPOINT ["/app/server"]
