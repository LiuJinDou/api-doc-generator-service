FROM golang:1.21-alpine AS builder

# Install git (required for cloning repositories)
RUN apk add --no-cache git

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o api-doc-generator ./cmd/server

# Final stage
FROM alpine:latest

# Install git and ca-certificates
RUN apk --no-cache add git ca-certificates

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/api-doc-generator .

# Create directory for cloned repositories
RUN mkdir -p /tmp/repos

EXPOSE 8080

CMD ["./api-doc-generator"]
