FROM golang:alpine AS builder

WORKDIR /app

# Копируем go.mod и go.sum из корня
COPY go.mod go.sum ./
RUN go mod download

# Копируем общий internal код
COPY internal/ ./internal/

# Копируем код сервиса
COPY services/notification/ ./services/notification/

# Собираем сервис
RUN go build -o notification ./services/notification/cmd/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/notification .

CMD ["./notification"]
