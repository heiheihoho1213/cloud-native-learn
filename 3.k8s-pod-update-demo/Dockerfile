FROM nginx:alpine

# 把本地html目录，复制到容器Nginx网页目录
COPY ./html /usr/share/nginx/html

# 复制自定义Nginx配置
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80