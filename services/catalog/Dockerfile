FROM golang:alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum from root
COPY go.mod go.sum ./
RUN go mod download

# Copy shared internal code
COPY internal/ ./internal/

# Copy service code
COPY services/catalog/ ./services/catalog/

# Build service
RUN go build -o catalog ./services/catalog/cmd/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/catalog .

CMD ["./catalog"]
