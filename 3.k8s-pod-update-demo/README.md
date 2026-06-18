> 目标：修改应用代码，触发一次滚动更新，观察新旧Pod的平滑切换过程。

# 将应用打包镜像
# 若使用 minikube，将镜像导入虚拟机（前提是 minikube start）。临时让当前终端 docker 指向 minikube 内部
eval $(minikube docker-env)
docker build -t my-nginx:v1 .

# 部署
kubectl apply -f nginx-deploy.yaml

# 查看情况
kubectl get deployments
kubectl get pods

# 访问服务
minikube service nginx-svc

# 监控 pod 变化
kubectl get pods -w

# 任意修改 index.html 后打包新的镜像
eval $(minikube docker-env)
docker build -t my-nginx:v2 .
修改 nginx-deploy.yaml 使用新的镜像

# 更新部署
kubectl apply -f nginx-deploy.yaml

此时可以观察 pod 监控界面的变化：
```
k8s-update-demo kubectl get pods -w
NAME                          READY   STATUS    RESTARTS   AGE
nginx-demo-7fcb87d9c4-dgh66   1/1     Running   0          42s
nginx-demo-7fcb87d9c4-hwvb2   1/1     Running   0          42s
nginx-demo-767655b877-nhtt2   0/1     Pending   0          0s
nginx-demo-767655b877-nhtt2   0/1     Pending   0          0s
nginx-demo-767655b877-nhtt2   0/1     ContainerCreating   0          0s
nginx-demo-767655b877-nhtt2   1/1     Running             0          1s
nginx-demo-7fcb87d9c4-hwvb2   1/1     Terminating         0          2m59s
nginx-demo-767655b877-d9lw6   0/1     Pending             0          0s
nginx-demo-7fcb87d9c4-hwvb2   1/1     Terminating         0          2m59s
nginx-demo-767655b877-d9lw6   0/1     Pending             0          0s
nginx-demo-767655b877-d9lw6   0/1     ContainerCreating   0          0s
nginx-demo-7fcb87d9c4-hwvb2   0/1     Completed           0          3m
nginx-demo-7fcb87d9c4-hwvb2   0/1     Completed           0          3m
nginx-demo-7fcb87d9c4-hwvb2   0/1     Completed           0          3m
nginx-demo-767655b877-d9lw6   1/1     Running             0          1s
nginx-demo-7fcb87d9c4-dgh66   1/1     Terminating         0          3m
nginx-demo-7fcb87d9c4-dgh66   1/1     Terminating         0          3m
nginx-demo-7fcb87d9c4-dgh66   0/1     Completed           0          3m1s
nginx-demo-7fcb87d9c4-dgh66   0/1     Completed           0          3m1s
nginx-demo-7fcb87d9c4-dgh66   0/1     Completed           0          3m1s
```

# 重新访问服务
minikube service nginx-svc