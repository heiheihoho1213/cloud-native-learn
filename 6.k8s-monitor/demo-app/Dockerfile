# 阶段1：编译 Go 二进制
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o demo-app .

# 阶段2：最小运行镜像
FROM alpine:3.19

WORKDIR /app
COPY --from=builder /app/demo-app .

EXPOSE 8080
CMD ["./demo-app"]
