FROM golang:alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum from root
COPY go.mod go.sum ./
RUN go mod download

# Copy shared internal code
COPY internal/ ./internal/

# Copy service code
COPY services/payment/ ./services/payment/

# Build service
RUN go build -o payment ./services/payment/cmd/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/payment .

CMD ["./payment"]
