FROM golang:alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum from root
COPY go.mod go.sum ./
RUN go mod download

# Copy shared internal code
COPY internal/ ./internal/

# Copy service code
COPY services/notification/ ./services/notification/

# Build service
RUN go build -o notification ./services/notification/cmd/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/notification .

CMD ["./notification"]
