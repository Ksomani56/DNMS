# Multi-stage Docker build for DNMS daemon
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build statically linked Linux binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/dnms .

# Minimal runtime image with raw socket networking tools
FROM alpine:3.20

RUN apk add --no-cache ca-certificates iproute2 iptables curl

WORKDIR /root/
COPY --from=builder /app/dnms /usr/local/bin/dnms
COPY --from=builder /app/dnms.example.yaml /etc/dnms/dnms.example.yaml

EXPOSE 12345 33434 33435/udp

ENTRYPOINT ["/usr/local/bin/dnms"]
CMD ["-httpAddr", ":12345"]
