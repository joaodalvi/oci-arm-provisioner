# Build Stage
FROM golang:alpine AS builder

WORKDIR /app

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

RUN go mod tidy
# Build
RUN go build -o oci-arm-provisioner ./main.go

# Runtime Stage
FROM alpine:latest

WORKDIR /app

# Install CA certs for HTTPS
RUN apk add --no-cache ca-certificates

# Copy binary
COPY --from=builder /app/oci-arm-provisioner .
# Copy example config for reference (optional)
COPY --from=builder /app/config.yaml.example .

# Run
CMD ["./oci-arm-provisioner"]
