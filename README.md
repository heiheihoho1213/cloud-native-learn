# 云原生技术学习实践指南

本指南通过 **6 个循序渐进的核心任务**，帮助你从零开始掌握云原生技术的核心实践：容器化、Kubernetes 编排、CI/CD 流水线、流量治理以及可观测性。

无论你是开发者还是运维工程师，完成这 6 个任务，你将具备构建、部署和生产级运维云原生应用的基本能力。

---

## 任务一：应用容器化 —— 构建并运行自定义镜像

**目标**：将你开发的一个 Web 应用（Go、Java、Python 或 Node.js）打包成 Docker 镜像，并在本地成功运行。

### 核心实践点
- 编写高质量的 `Dockerfile`（多阶段构建、层缓存优化）
- 使用 `.dockerignore` 排除无用文件
- 掌握 `docker build`, `docker run` 基本命令
- 验证容器内应用的端口映射与环境变量注入

### 预期产出
- 一个可运行的业务容器镜像
- 能够在本地通过 `http://localhost:8080` 访问你的应用

> 项目地址：<a href="./docker-demo">docker-demo</a>
---

## 任务二：集群上手 —— 部署 Nginx 并对外暴露服务

**目标**：拥有一个本地 Kubernetes 集群，并部署第一个 Nginx 应用，使其能够通过浏览器访问。

### 核心实践点
- 使用 `minikube`, `kind` 或 `K3s` 搭建单机集群
- 理解 `Pod`, `Deployment`, `Service` 的基本概念
- 使用 `kubectl create deployment` 与 `kubectl expose` 暴露服务
- 理解 `NodePort` 或 `LoadBalancer` 类型的 Service

### 预期产出
- 一个运行中的 Nginx Deployment（副本数 >= 1）
- 能够通过浏览器或 `curl` 访问 Nginx 欢迎页

> 项目地址：<a href="./k8s-demo">k8s-demo</a>
---

## 任务三：发布策略 —— 触发滚动更新并观测平滑切换

**目标**：修改应用代码或配置，触发一次滚动更新，并实时观察新旧 Pod 的平滑切换过程。

### 核心实践点
- 修改业务代码（如改变 API 返回内容），构建新版本镜像
- 执行 `kubectl set image` 或更新 Deployment YAML 触发滚动更新
- 使用 `kubectl get pods -w` 与 `kubectl rollout status` 实时监控切换过程
- 理解 `maxSurge`, `maxUnavailable` 对更新过程的影响
- 配置 `ReadinessProbe` 与 `preStop` 钩子确保零宕机发布

### 预期产出
- 一次无中断的应用版本升级过程
- 能够清楚描述新旧 Pod 替换过程中流量的切换逻辑

> 项目地址：<a href="./k8s-pod-update-demo">k8s-pod-update-demo</a>
---

## 任务四：自动化 —— 搭建端到端 CI/CD 流水线

**目标**：搭建一套从 Git Push 到服务自动更新的完整 CI/CD 流水线。

### 核心实践点
- 选择 CI/CD 工具（推荐 GitLab CI、GitHub Actions 或 Jenkins）
- 编排流水线步骤：代码检出 → 镜像构建 → 镜像推送 → 部署更新
- 使用 `imagePullPolicy: Always` 确保集群拉取最新镜像
- 使用 `kubectl rollout restart` 或直接 apply 新 YAML 触发更新

### 预期产出
- 一条自动化流水线，代码提交后自动完成全流程
- 验证流水线确实触发了 Kubernetes 中的 Pod 更新

---

## 任务五：流量治理 —— 微服务版本按权重路由

**目标**：使用 Go 语言启动两个微服务（v1 与 v2 版本），并实现按百分比切分的流量路由。

### 核心实践点
- 开发或准备两个版本的微服务（如 v1 返回 “hello v1”，v2 返回 “hello v2”）
- 使用 Kubernetes Ingress + Nginx Ingress Controller
- 利用 Ingress Annotations 或 Service Mesh（如 Istio）实现权重路由
- （推荐）使用 Istio 的 `VirtualService` 和 `DestinationRule` 完成精细的流量管理

### 预期产出
- 一个统一的访问入口，请求按比例（例如 v1:80%，v2:20%）自动分发到不同版本
- 能够通过连续访问验证流量分布是否符合预期

> 项目地址：<a href="./k8s-microapp-switch-demo">k8s-microapp-switch-demo</a>
---

## 任务六：可观测性 —— 排障模拟与链路分析

**目标**：模拟一个接口报错，通过 Grafana 看指标、Loki 查日志、Jaeger 分析链路，快速定位故障根源。

### 核心实践点
- 部署 LGTM 技术栈（Loki + Grafana + Tempo/Jaeger）或使用 Prometheus + Jaeger
- 在应用中埋点：日志输出、业务指标（如请求计数/错误率）、分布式链路追踪
- 模拟故障（如数据库超时、业务逻辑抛错）
- 串联排障流程：Grafana 发现异常 → Loki 查询错误日志 → Jaeger 定位慢调用或错误 Span

### 预期产出
- 一套完整的可观测性环境
- 清晰的故障定位路径：“通过指标发现异常 → 查看日志找到错误堆栈 → 追踪链路定位到具体代码行”

---

## 学习路径建议

| 阶段 | 任务 | 核心能力 |
| :--- | :--- | :--- |
| 初级 | 1, 2 | 容器化 + K8s 基础部署 |
| 中级 | 3, 4 | 发布策略 + CI/CD 自动化 |
| 高级 | 5, 6 | 流量治理 + 可观测性排障 |

