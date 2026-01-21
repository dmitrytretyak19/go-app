FROM golang:1.24-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY . .
RUN go build -o main .

FROM alpine:3.20
WORKDIR /root/
COPY --from=builder /app/main .
COPY --from=builder /app/*.go .
CMD ["./main"]