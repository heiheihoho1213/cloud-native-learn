> 目标：Go 简单微服务 + Kubernetes + Nginx 权重分流，部署 v1/v2 两个版本服务，按权重比例切分流量。

在项目根目录执行以下命令。

# 本地构建

```bash
# 在宿主机构建（先退出 minikube 的 docker 环境）
eval $(minikube docker-env -u)
docker build -t service-demo:v1 .
docker build -t service-demo:v2 .
```

v1/v2 镜像内容相同，版本差异由 Deployment 中的 `VERSION` 环境变量决定。

# 导入 minikube

```bash
# 若集群中已有同名旧镜像，先删除再导入
minikube ssh -- docker rmi -f service-demo:v1 service-demo:v2 2>/dev/null || true
minikube image load service-demo:v1
minikube image load service-demo:v2
```

# 一键部署

```bash
kubectl apply -f ./k8s/deploy-v1.yaml
kubectl apply -f ./k8s/deploy-v2.yaml
kubectl apply -f ./k8s/service.yaml
kubectl apply -f ./k8s/nginx-gateway.yaml
```

# 查看运行状态

```bash
kubectl get deploy,pod,svc
kubectl get deploy service-v1 service-v2
kubectl get svc svc-v1 svc-v2 nginx-gateway-svc
kubectl get pods -l app=go-service
```

# 测试权重分流

```bash
sh ./test.sh
```

结果：
Hello Go Service | Version: V2
Hello Go Service | Version: V1
Hello Go Service | Version: V2
Hello Go Service | Version: V1
Hello Go Service | Version: V2
Hello Go Service | Version: V1
Hello Go Service | Version: V2
Hello Go Service | Version: V1


脚本通过 `kubectl port-forward` 访问网关（macOS + Docker 驱动的 minikube 无法直接用 `minikube ip` 访问 NodePort），连续请求 30 次，观察 `Version: V1` / `Version: V2` 的出现比例。默认权重为 1:1，大致接近 50% : 50%。

# 单独验证各版本后端

```bash
# 在 Kubernetes 集群中临时启动一个交互式的 busybox 容器，用于调试或测试，退出后容器自动删除。
kubectl run test --image=busybox --rm -it --restart=Never -- sh
```

容器内执行：

```bash
wget -qO- svc-v1:8080
wget -qO- svc-v2:8080
```

# 调整权重比例

编辑 `k8s/nginx-gateway.yaml` 中 upstream 的 `weight` 值，例如 9:1 表示约 90% 流量到 v1：

```nginx
upstream backend {
    server svc-v1:8080 weight=9;
    server svc-v2:8080 weight=1;
}
```

重新生效：

```bash
kubectl apply -f ./k8s/nginx-gateway.yaml
kubectl rollout restart deployment nginx-gateway
sh ./test.sh
```

结果：
Hello Go Service | Version: V2
Hello Go Service | Version: V2
Hello Go Service | Version: V2
Hello Go Service | Version: V1
Hello Go Service | Version: V2
Hello Go Service | Version: V2
Hello Go Service | Version: V2
Hello Go Service | Version: V2
Hello Go Service | Version: V2

# 排查问题

```bash
# 确认后端是 Go 服务而非其他镜像（应看到 "Service start on :8080"）
kubectl logs deploy/service-v1 --tail=5

kubectl logs -l app=nginx-gateway
kubectl describe pod -l app=nginx-gateway
```

若 `test.sh` 返回 OpenResty 欢迎页，说明网关配置未生效，重新 apply 并重启：

```bash
kubectl apply -f ./k8s/nginx-gateway.yaml
kubectl rollout restart deployment nginx-gateway
```

若后端连接失败，确认在本项目目录重新构建并导入镜像后重启 Deployment：

```bash
eval $(minikube docker-env -u)
docker build -t service-demo:v1 .
docker build -t service-demo:v2 .
minikube image load service-demo:v1
minikube image load service-demo:v2
kubectl rollout restart deployment service-v1 service-v2
```

# 清理资源

```bash
kubectl delete -f ./k8s/nginx-gateway.yaml
kubectl delete -f ./k8s/service.yaml
kubectl delete -f ./k8s/deploy-v2.yaml
kubectl delete -f ./k8s/deploy-v1.yaml
```


> 注意事项：
> 1) macOS Docker 驱动下 minikube ip 不可达；
> 2) Nginx 配置挂载路径需配置正确；
> 3) 集群里的 service-demo 镜像是 nginx 而非 Go 服务。