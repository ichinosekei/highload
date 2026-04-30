FROM golang:alpine AS builder

WORKDIR /app

# Копируем go.mod и go.sum из корня
COPY go.mod go.sum ./
RUN go mod download

# Копируем общий internal код
COPY internal/ ./internal/

# Копируем код сервиса
COPY services/payment/ ./services/payment/

# Собираем сервис
RUN go build -o payment ./services/payment/cmd/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/payment .

CMD ["./payment"]
