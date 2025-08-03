FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY . .

RUN go mod tidy && go build -o events-service

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/events-service .

CMD ["./events-service"]
