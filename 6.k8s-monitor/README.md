> 说明：使用 K8s 部署对外接口，并模拟一个接口报错，通过 Grafana 看指标、Loki 查日志、Jaeger 分析链路，快速定位问题根源。

# K8s 可观测性实战指南

## 一、整体架构设计

### 1.1 技术栈选型

| 组件 | 作用 | 版本 |
|------|------|------|
| Prometheus | 指标采集与存储 | v2.45.0 |
| Grafana | 可视化与统一仪表盘 | 10.2.0 |
| Loki | 日志聚合与查询 | 2.9.1 |
| Promtail | 日志采集代理 | 2.9.1 |
| Jaeger | 分布式链路追踪 | 1.51.0 |
| OpenTelemetry | 统一遥测数据标准 | 1.0.0 |

### 1.2 数据流向

```
应用服务 → [指标] → Prometheus → Grafana
          → [日志] → Promtail → Loki → Grafana
          → [链路] → OTel Collector → Jaeger → Grafana
```

---

## 二、部署配置文件

### 2.1 创建命名空间

```yaml
# 00-namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: observability
  labels:
    name: observability
---
apiVersion: v1
kind: Namespace
metadata:
  name: demo-app
  labels:
    name: demo-app
```

### 2.2 Prometheus 部署

```yaml
# 01-prometheus.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: prometheus
  namespace: observability
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: prometheus
rules:
- apiGroups: [""]
  resources:
  - nodes
  - nodes/proxy
  - services
  - endpoints
  - pods
  verbs: ["get", "list", "watch"]
- apiGroups:
  - extensions
  resources:
  - ingresses
  verbs: ["get", "list", "watch"]
- nonResourceURLs: ["/metrics"]
  verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: prometheus
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: prometheus
subjects:
- kind: ServiceAccount
  name: prometheus
  namespace: observability
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-config
  namespace: observability
data:
  prometheus.yml: |
    global:
      scrape_interval: 15s
      evaluation_interval: 15s
    scrape_configs:
      - job_name: 'kubernetes-apiservers'
        kubernetes_sd_configs:
        - role: endpoints
        scheme: https
        tls_config:
          ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
        bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
        relabel_configs:
        - source_labels: [__meta_kubernetes_namespace, __meta_kubernetes_service_name, __meta_kubernetes_endpoint_port_name]
          action: keep
          regex: default;kubernetes;https
      
      - job_name: 'kubernetes-pods'
        kubernetes_sd_configs:
        - role: pod
        relabel_configs:
        - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
          action: keep
          regex: true
        - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
          action: replace
          target_label: __metrics_path__
          regex: (.+)
        - source_labels: [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
          action: replace
          regex: ([^:]+)(?::\d+)?;(\d+)
          replacement: $1:$2
          target_label: __address__
        - action: labelmap
          regex: __meta_kubernetes_pod_label_(.+)
        - source_labels: [__meta_kubernetes_namespace]
          action: replace
          target_label: kubernetes_namespace
        - source_labels: [__meta_kubernetes_pod_name]
          action: replace
          target_label: kubernetes_pod_name
      
      - job_name: 'demo-app'
        static_configs:
        - targets: ['demo-service.demo-app.svc:8080']
        metrics_path: '/metrics'
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: prometheus
  namespace: observability
spec:
  replicas: 1
  selector:
    matchLabels:
      app: prometheus
  template:
    metadata:
      labels:
        app: prometheus
    spec:
      serviceAccountName: prometheus
      containers:
      - name: prometheus
        image: prom/prometheus:v2.45.0
        args:
        - "--config.file=/etc/prometheus/prometheus.yml"
        - "--storage.tsdb.path=/prometheus/"
        ports:
        - containerPort: 9090
        volumeMounts:
        - name: prometheus-config-volume
          mountPath: /etc/prometheus/
        - name: prometheus-storage-volume
          mountPath: /prometheus/
      volumes:
      - name: prometheus-config-volume
        configMap:
          defaultMode: 420
          name: prometheus-config
      - name: prometheus-storage-volume
        emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: prometheus-service
  namespace: observability
spec:
  selector:
    app: prometheus
  type: NodePort
  ports:
  - port: 9090
    targetPort: 9090
    nodePort: 30090
```

### 2.3 Grafana 部署

```yaml
# 02-grafana.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: grafana-datasources
  namespace: observability
data:
  datasources.yaml: |
    apiVersion: 1
    datasources:
    - name: Prometheus
      type: prometheus
      url: http://prometheus-service.observability.svc:9090
      access: proxy
      isDefault: true
    - name: Loki
      type: loki
      url: http://loki-service.observability.svc:3100
      access: proxy
    - name: Jaeger
      type: jaeger
      url: http://jaeger-query.observability.svc:16686
      access: proxy
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: grafana
  namespace: observability
spec:
  replicas: 1
  selector:
    matchLabels:
      app: grafana
  template:
    metadata:
      labels:
        app: grafana
    spec:
      containers:
      - name: grafana
        image: grafana/grafana:10.2.0
        ports:
        - containerPort: 3000
        env:
        - name: GF_SECURITY_ADMIN_PASSWORD
          value: "admin123"
        volumeMounts:
        - name: grafana-datasources
          mountPath: /etc/grafana/provisioning/datasources
      volumes:
      - name: grafana-datasources
        configMap:
          name: grafana-datasources
---
apiVersion: v1
kind: Service
metadata:
  name: grafana-service
  namespace: observability
spec:
  selector:
    app: grafana
  type: NodePort
  ports:
  - port: 3000
    targetPort: 3000
    nodePort: 30030
```

### 2.4 Loki + Promtail 部署

```yaml
# 03-loki.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: loki-config
  namespace: observability
data:
  local-config.yaml: |
    auth_enabled: false
    server:
      http_listen_port: 3100
    common:
      path_prefix: /loki
      storage:
        filesystem:
          chunks_directory: /loki/chunks
          rules_directory: /loki/rules
      replication_factor: 1
      ring:
        instance_addr: 127.0.0.1
        kvstore:
          store: inmemory
    schema_config:
      configs:
        - from: 2020-10-24
          store: boltdb-shipper
          object_store: filesystem
          schema: v11
          index:
            prefix: index_
            period: 24h
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: loki
  namespace: observability
spec:
  replicas: 1
  selector:
    matchLabels:
      app: loki
  template:
    metadata:
      labels:
        app: loki
    spec:
      containers:
      - name: loki
        image: grafana/loki:2.9.1
        args:
        - "-config.file=/etc/loki/local-config.yaml"
        ports:
        - containerPort: 3100
        volumeMounts:
        - name: loki-config
          mountPath: /etc/loki
      volumes:
      - name: loki-config
        configMap:
          name: loki-config
---
apiVersion: v1
kind: Service
metadata:
  name: loki-service
  namespace: observability
spec:
  selector:
    app: loki
  ports:
  - port: 3100
    targetPort: 3100
---
# Promtail DaemonSet
apiVersion: v1
kind: ConfigMap
metadata:
  name: promtail-config
  namespace: observability
data:
  promtail.yaml: |
    server:
      http_listen_port: 9080
      grpc_listen_port: 0
    positions:
      filename: /tmp/positions.yaml
    clients:
      - url: http://loki-service.observability.svc:3100/loki/api/v1/push
    scrape_configs:
    - job_name: kubernetes-pods
      kubernetes_sd_configs:
      - role: pod
      relabel_configs:
      - source_labels:
        - __meta_kubernetes_pod_node_name
        target_label: __host__
      - action: labelmap
        regex: __meta_kubernetes_pod_label_(.+)
      - source_labels:
        - __meta_kubernetes_namespace
        target_label: namespace
      - source_labels:
        - __meta_kubernetes_pod_name
        target_label: pod
      - source_labels:
        - __meta_kubernetes_pod_container_name
        target_label: container
      - replacement: /var/log/pods/*$1/*.log
        separator: /
        source_labels:
        - __meta_kubernetes_pod_uid
        - __meta_kubernetes_pod_container_name
        target_label: __path__
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: promtail
  namespace: observability
spec:
  selector:
    matchLabels:
      app: promtail
  template:
    metadata:
      labels:
        app: promtail
    spec:
      containers:
      - name: promtail
        image: grafana/promtail:2.9.1
        args:
        - "-config.file=/etc/promtail/promtail.yaml"
        volumeMounts:
        - name: promtail-config
          mountPath: /etc/promtail
        - name: logs
          mountPath: /var/log
        - name: dockerlogs
          mountPath: /var/lib/docker/containers
          readOnly: true
      volumes:
      - name: promtail-config
        configMap:
          name: promtail-config
      - name: logs
        hostPath:
          path: /var/log
      - name: dockerlogs
        hostPath:
          path: /var/lib/docker/containers
```

### 2.5 Jaeger 部署

```yaml
# 04-jaeger.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: jaeger
  namespace: observability
spec:
  replicas: 1
  selector:
    matchLabels:
      app: jaeger
  template:
    metadata:
      labels:
        app: jaeger
    spec:
      containers:
      - name: jaeger
        image: jaegertracing/all-in-one:1.51
        ports:
        - containerPort: 16686  # Query UI
        - containerPort: 14268  # Collector HTTP
        - containerPort: 14250  # Collector gRPC
        - containerPort: 4317   # OTLP gRPC
        - containerPort: 4318   # OTLP HTTP
        env:
        - name: COLLECTOR_OTLP_ENABLED
          value: "true"
---
apiVersion: v1
kind: Service
metadata:
  name: jaeger-query
  namespace: observability
spec:
  selector:
    app: jaeger
  type: NodePort
  ports:
  - name: query
    port: 16686
    targetPort: 16686
    nodePort: 30086
---
apiVersion: v1
kind: Service
metadata:
  name: jaeger-collector
  namespace: observability
spec:
  selector:
    app: jaeger
  ports:
  - name: http
    port: 14268
    targetPort: 14268
  - name: grpc
    port: 14250
    targetPort: 14250
  - name: otlp-grpc
    port: 4317
    targetPort: 4317
  - name: otlp-http
    port: 4318
    targetPort: 4318
```

### 2.6 示例应用（含故障注入）

```yaml
# 05-demo-app.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo-app
  namespace: demo-app
spec:
  replicas: 2
  selector:
    matchLabels:
      app: demo-app
  template:
    metadata:
      labels:
        app: demo-app
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: "/metrics"
    spec:
      containers:
      - name: demo-app
        image: demo-app:latest
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 8080
        env:
        - name: ERROR_RATE
          value: "0.3"
        - name: SIMULATE_DELAY
          value: "true"
        - name: OTEL_EXPORTER_OTLP_ENDPOINT
          value: "jaeger-collector.observability.svc:4317"
        readinessProbe:
          httpGet:
            path: /api/health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        livenessProbe:
          httpGet:
            path: /api/health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 20
---
apiVersion: v1
kind: Service
metadata:
  name: demo-service
  namespace: demo-app
spec:
  selector:
    app: demo-app
  type: NodePort
  ports:
  - port: 8080
    targetPort: 8080
    nodePort: 30080
```

---

## 三、示例应用源代码（Go）

示例应用源码位于 `demo-app/` 目录，内置故障注入机制，并集成 Prometheus 指标与 OpenTelemetry 链路追踪。

### 3.1 创建项目文件

在 `k8s-monitor` 目录下执行：

```bash
mkdir -p demo-app
cd demo-app
```

需要创建 4 个文件：

```
k8s-monitor/demo-app/
├── main.go       # 应用主程序（HTTP 接口 + 故障注入 + 指标/链路）
├── go.mod        # Go 模块依赖
├── go.sum        # 依赖校验（go mod tidy 自动生成）
└── Dockerfile    # 容器镜像构建
```

**初始化模块并下载依赖：**

```bash
go mod init demo-app
go mod tidy
```

**`go.mod` 核心依赖：**

```go
module demo-app

go 1.25.0

require (
	github.com/prometheus/client_golang v1.23.2
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.69.0
	go.opentelemetry.io/otel v1.44.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.44.0
	go.opentelemetry.io/otel/sdk v1.44.0
	go.opentelemetry.io/otel/trace v1.44.0
)
```

完整 `main.go` 见仓库 [`demo-app/main.go`](demo-app/main.go)，核心接口如下：

```go
package main

import (
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type app struct {
	errorRate     float64  // 环境变量 ERROR_RATE，默认 0.3
	simulateDelay bool     // 环境变量 SIMULATE_DELAY，默认 true
	rng           *rand.Rand
}

func main() {
	a := &app{
		errorRate:     envFloat("ERROR_RATE", 0.3),
		simulateDelay: envBool("SIMULATE_DELAY", true),
		rng:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/hello", a.withMetrics(a.hello))
	mux.HandleFunc("GET /api/user/{id}", a.withMetrics(a.getUser))
	mux.HandleFunc("GET /api/order/{id}", a.withMetrics(a.getOrder))
	mux.HandleFunc("GET /api/health", a.withMetrics(a.health))
	mux.Handle("/metrics", promhttp.Handler()) // Prometheus 指标端点

	http.ListenAndServe(":8080", otelhttp.NewHandler(mux, "demo-app"))
}

func (a *app) getUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.log(r.Context(), slog.LevelInfo, "开始查询用户信息", "userId", id)

	if a.simulateDelay {
		time.Sleep(time.Duration(100+a.rng.Intn(400)) * time.Millisecond)
	}
	if a.rng.Float64() < a.errorRate {
		a.log(r.Context(), slog.LevelError, "查询用户失败: 数据库连接超时", "userId", id)
		http.Error(w, "数据库连接超时 - 模拟故障", http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "User: %s", id)
}
// getOrder、callInternalService、initTracer 等见完整源码
```

**`Dockerfile`：**

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o demo-app .

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/demo-app .
EXPOSE 8080
CMD ["./demo-app"]
```

### 3.2 本地运行

**方式一：直接运行（无需 Docker）**

```bash
cd k8s-monitor/demo-app

# 安装依赖
go mod tidy

# 启动服务（本地默认不连接 Jaeger，不会报错）
ERROR_RATE=0.3 SIMULATE_DELAY=true go run .

# 另开终端测试
curl http://localhost:8080/api/hello
curl http://localhost:8080/api/user/123
curl http://localhost:8080/api/health
curl http://localhost:8080/metrics
```

> **说明：** 本地运行时无需配置 Jaeger，日志中仍会输出 `traceId`，但不会尝试导出链路数据。如需本地连接 Jaeger，先 port-forward 再指定地址：
>
> ```bash
> kubectl port-forward -n observability svc/jaeger-collector 4317:4317
> OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317 go run .
> ```

**方式二：编译后运行**

```bash
go build -o demo-app .
./demo-app
```

**方式三：Docker 运行**

```bash
docker build -t demo-app:latest .
docker run -p 8080:8080 \
  -e ERROR_RATE=0.3 \
  -e SIMULATE_DELAY=true \
  demo-app:latest
```

### 3.3 部署到 K8s

```bash
cd k8s-monitor/demo-app

# 1. 构建镜像
docker build -t demo-app:latest .

# 2. 将镜像导入集群（本地集群需此步骤）
# kind:
kind load docker-image demo-app:latest
# minikube:
# minikube image load demo-app:latest

# 3. 部署应用（需先 apply 过 00-namespace.yaml）
kubectl apply -f 05-demo-app.yaml

# 4. 验证
kubectl get pods -n demo-app
kubectl port-forward -n demo-app svc/demo-service 8080:8080
curl http://localhost:8080/api/user/123
```

**环境变量说明：**

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `ERROR_RATE` | `0.3` | 故障注入概率（0~1） |
| `SIMULATE_DELAY` | `true` | 是否模拟数据库延迟 |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | 空（本地）/ K8s 中由 yaml 注入 | Jaeger OTLP 地址，未设置则不导出链路 |
| `OTEL_SDK_DISABLED` | `false` | 设为 `true` 强制关闭链路导出 |
| `PORT` | `8080` | 监听端口 |

---

## 四、部署后实战：模拟报错 → 看监控

> 目标：调用会随机报错的接口，在 Grafana 看到错误率上升、在 Loki 看到 ERROR 日志。

### 4.1 部署（k8s与minikube两种环境相同）

```bash
cd k8s-monitor

# ① 部署监控栈
kubectl apply -f 00-namespace.yaml \
  -f 01-prometheus.yaml \
  -f 02-grafana.yaml \
  -f 03-loki.yaml \
  -f 04-jaeger.yaml

# ② 构建并部署 demo 应用
cd demo-app && docker build -t demo-app:latest . && cd ..

# ③ 导入镜像（按你的集群选一条）
minikube image load demo-app:latest          # minikube
# kind load docker-image demo-app:latest    # kind
# Docker Desktop K8s 可跳过，与本机 Docker 共用镜像

kubectl apply -f 05-demo-app.yaml

# ④ 确认 Pod 全部 Running
kubectl get pods -n observability
kubectl get pods -n demo-app
```

---

### 4.2 方式一：K8s（Docker Desktop / 云集群）

Docker Desktop 内置 K8s、多数云集群的 **NodePort 可直接从本机访问**，无需 port-forward。

#### 访问地址

| 服务 | 地址 | 账号 |
|------|------|------|
| demo-app | http://127.0.0.1:30080 | - |
| Grafana | http://127.0.0.1:30030 | admin / admin123 |
| Prometheus | http://127.0.0.1:30090 | - |
| Jaeger | http://127.0.0.1:30086 | - |

云集群若 NodePort 不在本机，将 `127.0.0.1` 换成节点公网 IP：

```bash
NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="ExternalIP")].address}')
# 若无 ExternalIP，改用 InternalIP
[ -z "$NODE_IP" ] && NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')
echo "NODE_IP=$NODE_IP"
# 访问 http://$NODE_IP:30080 等
```

#### 部署后流程

```bash
# 1. 验证 demo-app
curl http://127.0.0.1:30080/api/health
# 期望：OK

# 2. 发请求，触发报错（约 30% 返回 500）
for i in $(seq 1 30); do
  curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:30080/api/user/$i
done

# 3. 打开 Grafana
open http://127.0.0.1:30030   # macOS；Linux 用 xdg-open
```

#### 看监控（Grafana 操作步骤）

登录 Grafana 后，去左侧 **Explore**（探索），不是首页 Dashboard。

**① 错误率：** 数据源选 **Prometheus** → 粘贴查询 → 时间选 **Last 5 minutes** → **Run query**

> **为啥全是 0？** `rate()` 只看「最近 1 分钟有没有新请求」。发完 30 次就停了，过 1 分钟 rate 会变 0。  
> **解决：** 下面终端 B 保持发流量，或发完立刻查；新手先用「累计错误数」更直观。

**推荐（发完请求立刻查，最简单）：**

```promql
sum(http_requests_total{status="500"})
```

**错误率（需持续发流量时看曲线）：**

```promql
sum(rate(http_requests_total{status="500"}[1m])) / sum(rate(http_requests_total{path="/api/user/{id}"}[1m]))
```

**② ERROR 日志：** 数据源切换 **Loki** → 粘贴 → **Run query**

> 注意 LogQL 语法：标签必须用花括号 `{namespace="demo-app"}`，不是 `namespace "demo-app"`。

```logql
{namespace="demo-app"} |= "ERROR"
```

若仍无日志，重新部署 Promtail（修复 RBAC）：

```bash
kubectl apply -f 03-loki.yaml
kubectl rollout restart -n observability daemonset/promtail
```

---

### 4.3 方式二：minikube

minikube（docker 驱动）在 macOS 上 **NodePort 默认不通**，部署后用 **port-forward** 访问。

#### 部署后流程

**终端 A**（保持运行）：

```bash
kubectl port-forward -n demo-app svc/demo-service 8080:8080 &
kubectl port-forward -n observability svc/grafana-service 3000:3000 &
```

**终端 B**（发请求）：

```bash
# 1. 验证
curl http://localhost:8080/api/health
# 期望：OK

# 2. 发请求，触发报错
for i in $(seq 1 30); do
  curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/api/user/$i
done

# 3. 打开 Grafana
open http://localhost:3000
```

#### 访问地址（port-forward 后）

| 服务 | 地址 | 账号 |
|------|------|------|
| demo-app | http://localhost:8080 | - |
| Grafana | http://localhost:3000 | admin / admin123 |

其他 UI 需要时再转发：

```bash
kubectl port-forward -n observability svc/prometheus-service 9090:9090   # Prometheus
kubectl port-forward -n observability svc/jaeger-query 16686:16686       # Jaeger
```

#### 看监控（Grafana 操作步骤）

发完 curl 后，登录 **http://localhost:3000**（`admin` / `admin123`），按下面两步看：

**① 看错误率（Prometheus 指标）**

1. 左侧菜单点 **Explore**（compass 图标 / 「探索」）
2. 顶部数据源下拉框选 **Prometheus**
3. 粘贴查询，时间选 **Last 5 minutes**，点 **Run query**

> **为啥全是 0？** `rate()` 只看最近 1 分钟有没有**新**请求。发完 30 次就停了，过 1 分钟会变 0。  
> **解决：** 发完立刻查；或终端 B 持续发流量（见下方）。

**推荐（发完 curl 立刻查即可）：**

```promql
sum(http_requests_total{status="500"})
```

有数字（例如 8~10）就说明 500 错误已被采集。

终端 持续发流量示例：

```bash
while true; do curl -s http://localhost:8080/api/user/$RANDOM > /dev/null; sleep 0.3; done
```

**② 看 ERROR 日志（Loki）**

1. 仍在 **Explore** 页面
2. 顶部数据源切换为 **Loki**
3. 输入（注意花括号，标签名是 `namespace`）：

```logql
{namespace="demo-app"} |= "ERROR"
```

4. 点 **Run query**，应看到「数据库连接超时」等 ERROR 日志

若 **No logs found**，先修复 Promtail 权限后重试：

```bash
kubectl apply -f 03-loki.yaml
kubectl rollout restart -n observability daemonset/promtail
# 等 30 秒，再发几条请求，然后重新查询
```

> minikube 也可用 NodePort：先运行 `minikube tunnel`（需 sudo），再用 `http://$(minikube ip):30080` 访问。一般不如 port-forward 省事。

---

### 4.4 两种方式对比

| | K8s（Docker Desktop / 云） | minikube |
|--|--|--|
| 访问 demo-app | `http://127.0.0.1:30080` | `http://localhost:8080`（port-forward） |
| 访问 Grafana | `http://127.0.0.1:30030` | `http://localhost:3000`（port-forward） |
| 是否需要 port-forward | 否 | 是 |
| 镜像导入 | 通常不需要 | `minikube image load demo-app:latest` |

**完成。** 看到 curl 输出混有 `200` 和 `500`，Grafana 错误率 > 0，Loki 有 ERROR 日志，即走完最小闭环。

---
