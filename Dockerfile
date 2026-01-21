FROM golang:1.24-alpine AS builder

# Устанавливаем git для go get
RUN apk add --no-cache git

# Устанавливаем зависимости
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Финальный образ
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /root/

# Копируем бинарник
COPY --from=builder /app/main .

# Запускаем приложение
CMD ["./main"]