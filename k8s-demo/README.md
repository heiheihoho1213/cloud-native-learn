> 目标：在电脑上拥有一个K8s集群，并部署第一个应用： 一个Nginx服务，并能通过浏览器访问到它。


# k8s 安装
下载适合当前操作系统版本的安装包 https://minikube.sigs.k8s.io/docs/start/?arch=%2Fmacos%2Farm64%2Fstable%2Fbinary+download

curl -LO https://github.com/kubernetes/minikube/releases/latest/download/minikube-darwin-arm64

sudo install minikube-darwin-arm64 /usr/local/bin/minikube

# 启动服务端
minikube start

# 检查版本和集群节点
kubectl version
kubectl get nodes

# 创建Pod yaml
新建文件 nginx-deploy.yaml

配置说明

| kind          | 核心用途                     | 适用场景                     | 生产是否常用 |
|---------------|------------------------------|------------------------------|--------------|
| Pod           | 最小运行单元，跑容器         | 临时调试、排错               | 极少单独用   |
| Deployment    | 管理无状态应用Pod            | Web服务、接口、普通业务程序  | ✅ 高频主力  |
| Service       | 固定访问入口、流量转发       | 所有需要对外/内部访问的服务  | ✅ 必用      |
| ConfigMap     | 存放明文配置                 | 配置文件、普通参数           | ✅ 常用      |
| Secret        | 存放密钥、密码等敏感信息     | AK/SK、账号密码、私有镜像密钥| ✅ 常用      |
| PVC           | 申请持久化存储               | 日志、数据库、文件存储       | ✅ 常用      |
| DaemonSet     | 每个节点部署一个Pod          | 监控、日志、节点代理         | ✅ 集群组件  |
| StatefulSet   | 管理有状态应用               | MySQL、Redis、中间件         | ✅ 中间件用  |
| Namespace     | 资源分组隔离                 | 多环境、多业务划分           | ✅ 必用      |


# 应用文件配置
kubectl apply -f nginx-deploy.yaml

# 查看部署情况

kubectl get deployments
kubectl get pods

# 查看端口情况
kubectl get svc nginx-svc

> 如果使用的是 minikube，则端口是mini虚拟机的端口

# 访问nginx
minikube service nginx-svc

> 临时启动 pod 拍错：kubectl port-forward pod/nginx-demo-86644db9cc-xjrr8 8080:80

# 关闭服务
kubectl delete -f nginx-deploy.yaml