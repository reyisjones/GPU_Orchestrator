# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /workspace

# Copy go mod and sum files
COPY go.mod go.mod
COPY go.sum go.sum

# Cache deps before building and copying source to be able to use the Docker cache
RUN go mod download

# Copy the go source
COPY cmd/ cmd/
COPY api/ api/
COPY controllers/ controllers/
COPY internal/ internal/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags="-w -s" -o manager cmd/manager/main.go

# Final stage - minimal image
FROM alpine:3.18

RUN apk add --no-cache ca-certificates

WORKDIR /

# Copy the binary from builder
COPY --from=builder /workspace/manager .

# Create non-root user
RUN addgroup -g 65532 nonroot && \
    adduser -u 65532 -G nonroot -s /sbin/nologin -D nonroot

USER 65532:65532

EXPOSE 8080 8081 9443

ENTRYPOINT ["/manager"]
