FROM golang:alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum from root
COPY go.mod go.sum ./
RUN go mod download

# Copy shared internal code
COPY internal/ ./internal/

# Copy service code
COPY services/order/ ./services/order/

# Build service
RUN go build -o order ./services/order/cmd/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/order .

CMD ["./order"]
