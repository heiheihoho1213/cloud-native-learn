> 目标：将一个自己写过的Web应用（比如Go或Java程序）打包成镜像，并成功在本地运行起来


# 格式：docker build -t 镜像名:标签 .
docker build -t go-web-demo:v1 .

# 查看镜像
docker images

# 端口映射：主机8080 映射到 容器8080
docker run -d -p 8080:8080 --name go-web-container go-web-demo:v1