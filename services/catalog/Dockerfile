FROM golang:alpine AS builder

WORKDIR /app

# Копируем go.mod и go.sum из корня
COPY go.mod go.sum ./
RUN go mod download

# Копируем общий internal код
COPY internal/ ./internal/

# Копируем код сервиса
COPY services/catalog/ ./services/catalog/

# Собираем сервис
RUN go build -o catalog ./services/catalog/cmd/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/catalog .

CMD ["./catalog"]
