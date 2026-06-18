FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod ./
COPY main.go ./
ARG APP_VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.Version=${APP_VERSION}" -o app .

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/app .
EXPOSE 8080
CMD ["./app"]
