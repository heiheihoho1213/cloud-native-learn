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
        metrics_path: '/actuator/prometheus'
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
apiVersion: v1
kind: ConfigMap
metadata:
  name: demo-app-config
  namespace: demo-app
data:
  application.properties: |
    # Server
    server.port=8080
    
    # Metrics
    management.endpoints.web.exposure.include=*
    management.endpoint.prometheus.enabled=true
    management.metrics.tags.application=${spring.application.name}
    
    # Logging
    logging.pattern.console=%d{yyyy-MM-dd HH:mm:ss.SSS} [%thread] [%X{traceId:-},%X{spanId:-}] %-5level %logger{36} - %msg%n
    
    # OpenTelemetry
    otel.service.name=demo-app
    otel.traces.exporter=otlp
    otel.metrics.exporter=none
    otel.logs.exporter=none
    otel.exporter.otlp.endpoint=http://jaeger-collector.observability.svc:4317
---
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
        prometheus.io/path: "/actuator/prometheus"
    spec:
      containers:
      - name: demo-app
        image: openjdk:17-jdk-slim
        ports:
        - containerPort: 8080
        command: ["java"]
        args:
        - "-javaagent:/opentelemetry-javaagent.jar"
        - "-jar"
        - "/app/demo-app.jar"
        volumeMounts:
        - name: app-config
          mountPath: /config
        env:
        - name: SPRING_CONFIG_LOCATION
          value: /config/application.properties
        - name: ERROR_RATE
          value: "0.3"
        - name: SIMULATE_DELAY
          value: "true"
        readinessProbe:
          httpGet:
            path: /actuator/health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        livenessProbe:
          httpGet:
            path: /actuator/health
            port: 8080
          initialDelaySeconds: 60
          periodSeconds: 20
      initContainers:
      - name: download-agent
        image: curlimages/curl:latest
        command:
        - sh
        - -c
        - |
          curl -L -o /opentelemetry-javaagent.jar https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar
          curl -L -o /app/demo-app.jar https://github.com/xxx/demo-app.jar  # 使用你自己的应用 jar
        volumeMounts:
        - name: otel-agent
          mountPath: /opentelemetry-javaagent.jar
        - name: app-jar
          mountPath: /app
      volumes:
      - name: app-config
        configMap:
          name: demo-app-config
      - name: otel-agent
        emptyDir: {}
      - name: app-jar
        emptyDir: {}
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

## 三、示例应用源代码（Spring Boot）

为了让演示更完整，这里提供一个完整的 Spring Boot 应用代码，内置故障注入机制：

```java
package com.example.demo;

import io.opentelemetry.api.trace.Span;
import io.opentelemetry.api.trace.Tracer;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.slf4j.MDC;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RestController;

import java.util.Random;

@SpringBootApplication
@RestController
public class DemoApplication {
    
    private static final Logger logger = LoggerFactory.getLogger(DemoApplication.class);
    private final Random random = new Random();
    
    @Value("${ERROR_RATE:0.3}")
    private double errorRate;
    
    @Value("${SIMULATE_DELAY:true}")
    private boolean simulateDelay;

    public static void main(String[] args) {
        SpringApplication.run(DemoApplication.class, args);
    }

    @GetMapping("/api/hello")
    public String hello() {
        logger.info("收到 hello 请求");
        return "Hello, World!";
    }

    @GetMapping("/api/user/{id}")
    public String getUser(@PathVariable String id) throws Exception {
        String traceId = MDC.get("traceId");
        logger.info("开始查询用户信息, userId={}, traceId={}", id, traceId);
        
        // 模拟延迟
        if (simulateDelay) {
            int delayMs = 100 + random.nextInt(400);
            Thread.sleep(delayMs);
            logger.debug("数据库查询耗时: {}ms", delayMs);
        }
        
        // 按概率注入错误
        if (random.nextDouble() < errorRate) {
            logger.error("查询用户失败: 数据库连接超时, userId={}", id);
            throw new RuntimeException("数据库连接超时 - 模拟故障");
        }
        
        // 按概率注入慢查询
        if (random.nextDouble() < 0.2) {
            logger.warn("检测到慢查询, userId={}", id);
            Thread.sleep(2000);
        }
        
        logger.info("用户查询成功, userId={}", id);
        return "User: " + id;
    }

    @GetMapping("/api/order/{id}")
    public String getOrder(@PathVariable String id) throws Exception {
        logger.info("开始查询订单信息, orderId={}", id);
        
        // 调用内部服务
        callInternalService();
        
        // 更高概率的错误
        if (random.nextDouble() < errorRate * 1.5) {
            logger.error("订单服务异常: 支付网关响应超时, orderId={}", id);
            throw new RuntimeException("支付网关响应超时 - 模拟故障");
        }
        
        return "Order: " + id;
    }

    private void callInternalService() throws InterruptedException {
        logger.info("调用内部依赖服务");
        Thread.sleep(50 + random.nextInt(100));
        
        if (random.nextDouble() < 0.1) {
            logger.warn("内部服务调用告警: 响应时间超过阈值");
        }
    }

    @GetMapping("/api/health")
    public String health() {
        return "OK";
    }
}
```

**pom.xml 依赖配置：**

```xml
<dependencies>
    <dependency>
        <groupId>org.springframework.boot</groupId>
        <artifactId>spring-boot-starter-web</artifactId>
    </dependency>
    <dependency>
        <groupId>org.springframework.boot</groupId>
        <artifactId>spring-boot-starter-actuator</artifactId>
    </dependency>
    <dependency>
        <groupId>io.micrometer</groupId>
        <artifactId>micrometer-registry-prometheus</artifactId>
    </dependency>
</dependencies>
```

---

## 四、操作步骤

### 4.1 部署可观测性栈

```bash
# 1. 创建命名空间
kubectl apply -f 00-namespace.yaml

# 2. 部署 Prometheus
kubectl apply -f 01-prometheus.yaml

# 3. 部署 Grafana
kubectl apply -f 02-grafana.yaml

# 4. 部署 Loki + Promtail
kubectl apply -f 03-loki.yaml

# 5. 部署 Jaeger
kubectl apply -f 04-jaeger.yaml

# 6. 验证所有 Pod 运行状态
kubectl get pods -n observability
```

### 4.2 部署示例应用

```bash
# 构建并推送应用镜像（可选，使用预构建镜像可跳过）
# mvn clean package -DskipTests
# docker build -t demo-app:latest .
# docker push demo-app:latest

# 部署应用
kubectl apply -f 05-demo-app.yaml

# 验证应用 Pod
kubectl get pods -n demo-app
```

### 4.3 生成测试流量（故障触发）

```bash
# 1. 获取 NodePort
NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[0].address}')

# 2. 持续发送请求触发故障
while true; do
  curl -s http://$NODE_IP:30080/api/user/$RANDOM > /dev/null 2>&1
  curl -s http://$NODE_IP:30080/api/order/$RANDOM > /dev/null 2>&1
  sleep 0.5
done &

# 3. 也可以使用 hey 压测工具
hey -z 5m -c 10 -q 2 http://$NODE_IP:30080/api/user/123
```

### 4.4 访问各系统 UI

| 系统 | 地址 | 账号 / 密码 |
|------|------|-------------|
| Grafana | http://NodeIP:30030 | admin / admin123 |
| Prometheus | http://NodeIP:30090 | - |
| Jaeger | http://NodeIP:30086 | - |

---

## 五、问题定位实战演示

### 5.1 场景：接口报错率突增，用户反馈系统异常

#### 第一步：Grafana 指标发现异常

**操作路径：** Grafana → Explore → Prometheus

**查询异常指标：**

```promql
# 1. 查看 HTTP 请求错误率
rate(http_server_requests_seconds_count{status=~"5.."}[5m]) 
/ 
rate(http_server_requests_seconds_count[5m])

# 2. 按接口查看错误分布
sum by (uri) (rate(http_server_requests_seconds_count{status=~"5.."}[5m]))

# 3. P95 响应时间
histogram_quantile(0.95, sum by (le, uri) (rate(http_server_requests_seconds_bucket[5m])))
```

**发现的问题：**

- `/api/user/{id}` 接口错误率达到 30%
- `/api/order/{id}` 接口错误率达到 45%
- P95 响应时间从 200ms 飙升到 2.5s

**关键结论：** 系统确实存在异常，错误集中在特定接口

---

#### 第二步：Loki 日志定位错误详情

**操作路径：** Grafana → Explore → Loki

**关联查询（从指标跳转到日志）：**

从 Prometheus 图表点击 → "View logs" 自动跳转

**日志查询语句：**

```logql
# 1. 查询 ERROR 级别日志
{namespace="demo-app"} |= "ERROR"

# 2. 按错误类型统计
count_over_time({namespace="demo-app"} |= "数据库连接超时" [5m])

# 3. 查看特定 traceId 的完整日志
{namespace="demo-app"} |= "689f54d7a8e2b1c3"

# 4. 错误日志上下文分析
{namespace="demo-app"} 
  | json 
  | level = "ERROR"
  | line_format "{{.timestamp}} {{.logger}} - {{.message}}"
```

**日志分析发现：**

```
2024-01-15 10:30:45.123 [http-nio-8080-exec-3] [689f54d7a8e2b1c3,7a2d1f4e5b6c8d9a] ERROR com.example.demo.DemoApplication - 查询用户失败: 数据库连接超时, userId=45678

2024-01-15 10:30:46.456 [http-nio-8080-exec-7] [3f7a2d1c9b8e4f5a,2c3d4e5f6a7b8c9d] ERROR com.example.demo.DemoApplication - 订单服务异常: 支付网关响应超时, orderId=12345
```

**关键发现：**

1. 错误类型主要有两种：数据库连接超时、支付网关响应超时
2. 每个错误日志都带有 traceId
3. 错误发生频率与指标显示一致

---

#### 第三步：Jaeger 链路追踪根因分析

**操作路径：** Grafana → Explore → Jaeger

**使用日志中的 traceId 直接查询：**

**Trace: `689f54d7a8e2b1c3` 完整链路：**

```
┌─────────────────────────────────────────────────────────────┐
│ GET /api/user/45678                             2,345ms  ✘ │
├─────────────────────────────────────────────────────────────┤
│ ├── DemoApplication.getUser                     2,345ms  ✘ │
│ │   ├── 数据库查询                                2,000ms    │
│ │   └── 日志记录                                     5ms    │
└─────────────────────────────────────────────────────────────┘
```

**Tags:**

- error: true
- http.status_code: 500
- db.type: mysql
- exception.message: "数据库连接超时 - 模拟故障"

**Events:**

1. 1.000s: "开始获取数据库连接"
2. 2.000s: "获取连接超时，抛出异常"
3. 2.345s: "请求处理完成，返回 500"

**Trace: `3f7a2d1c9b8e4f5a` 完整链路：**

```
┌─────────────────────────────────────────────────────────────┐
│ GET /api/order/12345                            5,678ms  ✘ │
├─────────────────────────────────────────────────────────────┤
│ ├── DemoApplication.getOrder                    5,678ms  ✘ │
│ │   ├── callInternalService                        150ms    │
│ │   ├── 支付网关调用                             5,500ms    │
│ │   └── 日志记录                                     28ms    │
└─────────────────────────────────────────────────────────────┘
```

**Tags:**

- error: true
- http.status_code: 500
- external.service: payment-gateway
- exception.message: "支付网关响应超时 - 模拟故障"

**链路分析结论：**

| 问题类型 | 根因 | 影响范围 |
|----------|------|----------|
| 数据库连接超时 | 数据库连接池耗尽，获取连接等待超过 2s | `/api/user/*` 接口 |
| 支付网关超时 | 第三方支付服务响应缓慢，超时 5s+ | `/api/order/*` 接口 |

---

### 5.2 三者联动排查思路总结

**标准排查流程：**

1. **【Grafana 指标层】发现异常**
   - 现象：错误率↑、延迟↑、吞吐量↓
   - 方法：RED 方法论（Rate、Errors、Duration）

2. **【Loki 日志层】定位错误类型**
   - 现象：ERROR 日志、异常堆栈、具体错误消息
   - 方法：根据 traceId 关联、按错误类型聚合

3. **【Jaeger 链路层】分析根因**
   - 现象：哪个服务/数据库/外部调用慢
   - 方法：瀑布图分析、关键路径识别、Tag 详情

4. **【问题修复】**
   - 数据库：扩容连接池、优化 SQL、增加缓存
   - 外部依赖：增加熔断、降级、重试机制

**联动关键技巧：**

1. 统一 TraceID：日志、指标、链路使用相同的 traceId
2. Grafana 关联跳转：
   - 指标图表 → 点击跳转到对应日志
   - 日志中的 traceId → 点击跳转到 Jaeger 链路
3. 统一标签：namespace、pod、service、version 等标签一致

---

## 六、问题修复建议

### 6.1 数据库连接超时修复

**配置调整：**

```properties
# application.properties
spring.datasource.hikari.maximum-pool-size=20
spring.datasource.hikari.minimum-idle=10
spring.datasource.hikari.connection-timeout=30000
spring.datasource.hikari.max-lifetime=1800000
```

**代码优化：**

- 增加 Redis 缓存层
- 优化慢查询 SQL
- 增加数据库读写分离

### 6.2 外部服务超时修复

**增加 Resilience4j 熔断降级：**

```java
@CircuitBreaker(name = "paymentService", fallbackMethod = "paymentFallback")
@Retry(name = "paymentService")
@TimeLimiter(name = "paymentService")
public String callPaymentService(String orderId) {
    // 调用支付网关
}

public String paymentFallback(String orderId, Exception ex) {
    logger.warn("支付服务降级, orderId={}", orderId);
    return "支付处理中，请稍后查询";
}
```

### 6.3 验证修复效果

```promql
# 1. 观察错误率是否下降
rate(http_server_requests_seconds_count{status=~"5.."}[5m])

# 2. 观察 P95 延迟是否恢复正常
histogram_quantile(0.95, sum by (le) (rate(http_server_requests_seconds_bucket[5m])))

# 3. 确认 ERROR 日志消失
count_over_time({namespace="demo-app"} |= "ERROR" [5m])
```

---

## 七、关键配置检查清单

### TraceId 贯穿三系统

- [x] 日志格式包含 `%X{traceId}`
- [x] OpenTelemetry Agent 正确配置
- [x] Jaeger Collector 接收 OTLP 数据

### 指标采集完整

- [x] `/actuator/prometheus` 端点暴露
- [x] Prometheus 配置正确的 scrape job
- [x] Pod 注解包含 `prometheus.io/scrape`

### 日志采集正常

- [x] Promtail DaemonSet 在每个节点运行
- [x] Loki 接收并索引日志
- [x] 日志包含 namespace、pod 等标签

---

**实战完成！** 通过这套完整的可观测性方案，你可以在 5 分钟内从「系统异常」的告警，精准定位到具体的代码问题和依赖瓶颈。
