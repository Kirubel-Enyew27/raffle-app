# ---- Build stage ----
FROM golang:1.25-alpine AS builder

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Cache module downloads
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Build with CGO disabled for static binary
COPY backend/ .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/server ./cmd/server

# ---- Run stage ----
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary and migrations
COPY --from=builder /app/server .
COPY backend/migrations/ ./migrations/

EXPOSE 8080

CMD ["./server"]
