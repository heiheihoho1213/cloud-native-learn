#!/usr/bin/env bash
# 本地模拟 CI/CD：构建镜像 → 导入集群 → 更新 Deployment
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${1:-$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || echo "local")}"

echo "==> 版本: $VERSION"

# minikube：在虚拟机内构建镜像（无需 push 到远程仓库）
if command -v minikube >/dev/null 2>&1 && minikube status >/dev/null 2>&1; then
  echo "==> 使用 minikube docker-env 构建"
  eval "$(minikube docker-env)"
fi

echo "==> 构建镜像"
docker build -t "cicd-demo:${VERSION}" --build-arg "APP_VERSION=${VERSION}" "$ROOT/app"
docker tag "cicd-demo:${VERSION}" cicd-demo:latest

echo "==> 部署到 K8s"
kubectl apply -f "$ROOT/k8s/deploy.yaml"
kubectl set image deployment/cicd-demo "app=cicd-demo:${VERSION}" -n cicd-demo
kubectl set env deployment/cicd-demo "APP_VERSION=${VERSION}" -n cicd-demo
kubectl rollout status deployment/cicd-demo -n cicd-demo --timeout=120s

echo "==> 完成！验证："
echo "    kubectl port-forward -n cicd-demo svc/cicd-demo 8080:8080"
echo "    curl http://localhost:8080"
