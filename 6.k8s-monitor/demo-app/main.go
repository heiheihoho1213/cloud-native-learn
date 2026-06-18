package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// HTTP 指标：供 Prometheus 采集
var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "HTTP 请求总数",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP 请求耗时分布",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)

type app struct {
	errorRate     float64
	simulateDelay bool
	rng           *rand.Rand
}

func main() {
	// 初始化 OpenTelemetry，将链路数据发送到 Jaeger
	shutdown, err := initTracer()
	if err != nil {
		slog.Error("初始化链路追踪失败", "error", err)
		os.Exit(1)
	}
	defer shutdown(context.Background())

	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration)

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
	mux.Handle("/metrics", promhttp.Handler())

	port := envString("PORT", "8080")
	slog.Info("服务启动", "port", port)
	if err := http.ListenAndServe(":"+port, otelhttp.NewHandler(mux, "demo-app")); err != nil {
		slog.Error("服务异常退出", "error", err)
		os.Exit(1)
	}
}

func (a *app) hello(w http.ResponseWriter, r *http.Request) {
	a.log(r.Context(), slog.LevelInfo, "收到 hello 请求")
	fmt.Fprint(w, "Hello, World!")
}

func (a *app) getUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.log(r.Context(), slog.LevelInfo, "开始查询用户信息", "userId", id)

	if a.simulateDelay {
		delay := time.Duration(100+a.rng.Intn(400)) * time.Millisecond
		time.Sleep(delay)
		a.log(r.Context(), slog.LevelDebug, "数据库查询耗时", "delayMs", delay.Milliseconds())
	}

	if a.rng.Float64() < a.errorRate {
		a.log(r.Context(), slog.LevelError, "查询用户失败: 数据库连接超时", "userId", id)
		http.Error(w, "数据库连接超时 - 模拟故障", http.StatusInternalServerError)
		return
	}

	if a.rng.Float64() < 0.2 {
		a.log(r.Context(), slog.LevelWarn, "检测到慢查询", "userId", id)
		time.Sleep(2 * time.Second)
	}

	a.log(r.Context(), slog.LevelInfo, "用户查询成功", "userId", id)
	fmt.Fprintf(w, "User: %s", id)
}

func (a *app) getOrder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.log(r.Context(), slog.LevelInfo, "开始查询订单信息", "orderId", id)

	a.callInternalService(r.Context())

	if a.rng.Float64() < a.errorRate*1.5 {
		a.log(r.Context(), slog.LevelError, "订单服务异常: 支付网关响应超时", "orderId", id)
		http.Error(w, "支付网关响应超时 - 模拟故障", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Order: %s", id)
}

func (a *app) callInternalService(ctx context.Context) {
	tracer := otel.Tracer("demo-app")
	ctx, span := tracer.Start(ctx, "callInternalService")
	defer span.End()

	a.log(ctx, slog.LevelInfo, "调用内部依赖服务")
	time.Sleep(time.Duration(50+a.rng.Intn(100)) * time.Millisecond)

	if a.rng.Float64() < 0.1 {
		a.log(ctx, slog.LevelWarn, "内部服务调用告警: 响应时间超过阈值")
	}
}

func (a *app) health(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "OK")
}

// withMetrics 记录 Prometheus 指标
func (a *app) withMetrics(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next(rw, r)

		path := normalizePath(r)
		httpRequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(rw.status)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, path).Observe(time.Since(start).Seconds())
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// normalizePath 将 /api/user/123 归一化为 /api/user/{id}，便于 Prometheus 聚合
func normalizePath(r *http.Request) string {
	p := r.URL.Path
	if strings.HasPrefix(p, "/api/user/") {
		return "/api/user/{id}"
	}
	if strings.HasPrefix(p, "/api/order/") {
		return "/api/order/{id}"
	}
	return p
}

// log 输出带 traceId 的结构化日志，便于 Loki 关联查询
func (a *app) log(ctx context.Context, level slog.Level, msg string, args ...any) {
	traceID := ""
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		traceID = span.SpanContext().TraceID().String()
	}
	args = append(args, "traceId", traceID)
	slog.Log(ctx, level, msg, args...)
}

func initTracer() (func(context.Context) error, error) {
	// 本地开发默认不导出链路；K8s 部署通过环境变量指定 Jaeger 地址
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	disabled := envBool("OTEL_SDK_DISABLED", false)

	resourceOpts := sdktrace.WithResource(resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("demo-app"),
	))

	if disabled || endpoint == "" {
		// 不配置 exporter：仍生成 traceId 供日志关联，但不会尝试连接 Jaeger
		slog.Info("链路追踪导出已关闭（本地模式）", "hint", "如需导出请设置 OTEL_EXPORTER_OTLP_ENDPOINT")
		tp := sdktrace.NewTracerProvider(resourceOpts)
		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(propagation.TraceContext{})
		return tp.Shutdown, nil
	}

	exporter, err := otlptracegrpc.New(
		context.Background(),
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		resourceOpts,
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	slog.Info("链路追踪已启用", "endpoint", endpoint)

	return tp.Shutdown, nil
}

func envString(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envFloat(key string, defaultVal float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return defaultVal
	}
	return f
}

func envBool(key string, defaultVal bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return defaultVal
	}
	return b
}
