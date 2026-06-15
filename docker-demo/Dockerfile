# 阶段1：编译代码（构建镜像）
FROM golang:1.24-alpine AS builder

# 设置工作目录
WORKDIR /app

# 拷贝源码
COPY . .

# 编译二进制程序，关闭 CGO、静态编译
RUN CGO_ENABLED=0 GOOS=linux go build -o webapp main.go

# 阶段2：运行镜像（仅保留可执行文件，极简镜像）
FROM alpine:3.19

WORKDIR /app

# 从构建阶段拷贝编译好的程序
COPY --from=builder /app/webapp .

# 暴露容器端口（仅声明，不实际开启端口映射）
EXPOSE 8080

# 容器启动命令
CMD ["./webapp"]