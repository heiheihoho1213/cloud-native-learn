> 说明：搭建一套你自己的 CI/CD 流水线，验证从 git push 到 K8s 服务自动更新的全过程。

# CI/CD 实战

## 流程一览

```
改代码 → git push → CI 流水线触发（GitHub Actions / GitLab CI）
         → 构建 Docker 镜像 → 更新 K8s Deployment → 滚动发布新版本
```

## 目录结构

```
4.cicd-demo/
├── app/                         # Go 应用
├── k8s/deploy.yaml              # K8s 部署清单
├── scripts/deploy.sh            # 统一部署脚本（CI 和本地共用）
├── .github/workflows/deploy.yml # GitHub Actions
└── .gitlab-ci.yml               # GitLab CI
```

> 建议将 `4.cicd-demo` 作为独立 Git 仓库 push 到 GitHub / GitLab。若放在大仓库子目录，需在 CI 平台配置工作目录。

---

## 方式一：本地模拟（先跑通，5 分钟）

```bash
minikube start
cd 4.cicd-demo

# 首次部署
./scripts/deploy.sh v1

# 访问验证
kubectl port-forward -n cicd-demo svc/cicd-demo 8080:8080
curl http://localhost:8080
# → CI/CD Demo - version: v1

# 改 app/main.go 后再次发布
./scripts/deploy.sh v2
curl http://localhost:8080
# → CI/CD Demo - version: v2
```

---

## 方式二：GitHub Actions

### 配置文件

[`.github/workflows/deploy.yml`](.github/workflows/deploy.yml)

### 前置条件

- 代码推到 **GitHub** 仓库
- 本机 `minikube` 已启动
- 已注册 **自托管 Runner**（云上 Runner 访问不到本机 minikube）

### 配置步骤

**① 注册 Runner**

GitHub 仓库 → **Settings** → **Actions** → **Runners** → **New self-hosted runner**

**② 首次手动部署（只需一次）**

```bash
./scripts/deploy.sh v1
```

**③ push 触发**

```bash
git add .
git commit -m "feat: update app"
git push
```

**④ 查看结果**

GitHub → **Actions** → **CI/CD Deploy** 变绿即成功。

```bash
curl http://localhost:8080   # 需 port-forward
# version 变为 commit 短哈希，如 a1b2c3d
```

---

## 方式三：GitLab CI

### 配置文件

[`.gitlab-ci.yml`](.gitlab-ci.yml)

### 前置条件

- 代码推到 **GitLab** 仓库
- 本机 `minikube` 已启动
- 已注册 **自托管 GitLab Runner**，标签为 `minikube`

### 配置步骤

**① 安装并注册 Runner**

```bash
# Linux（以 Ubuntu/Debian 为例）
curl -L "https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.deb.sh" | sudo bash
sudo apt-get install gitlab-runner

# 注册 Runner
sudo gitlab-runner register
```

注册时填写：

| 配置项 | 取值 |
|--------|------|
| GitLab URL | 你的 GitLab 地址 |
| Token | 项目 Settings → CI/CD → Runners |
| Executor | `shell` |
| Tags | `minikube` |

启动 Runner：

```bash
sudo gitlab-runner run
```

**② 首次手动部署（只需一次）**

```bash
./scripts/deploy.sh v1
```

**③ push 触发**

```bash
git add .
git commit -m "feat: update app"
git push origin main
```

**④ 查看结果**

GitLab → **Build** → **Pipelines** → `deploy` job 成功。

---

## 两种 CI 对比

| | GitHub Actions | GitLab CI |
|--|--|--|
| 配置文件 | `.github/workflows/deploy.yml` | `.gitlab-ci.yml` |
| 自托管 Runner | Settings → Actions → Runners | `gitlab-runner register` |
| Runner 标签 | `runs-on: self-hosted` | `tags: [minikube]` |
| 版本号变量 | `GITHUB_SHA` | `CI_COMMIT_SHORT_SHA` |
| 查看流水线 | Actions 页 | Pipelines 页 |

两者最终都调用同一个脚本：

```bash
./scripts/deploy.sh <版本号>
```

---

## 流水线做了什么？

| 步骤 | 等价命令 |
|------|---------|
| 构建镜像 | `docker build -t cicd-demo:xxx ./app` |
| 更新部署 | `kubectl set image deployment/cicd-demo app=cicd-demo:xxx` |
| 等待完成 | `kubectl rollout status deployment/cicd-demo` |

---

## 常见问题

**Q：为什么必须用自托管 Runner？**

本机 minikube 只有你的电脑能访问，云上 Runner 连不上。

**Q：Runner 报错找不到 kubectl？**

确保 minikube 已启动，且 Runner 进程能读到 `~/.kube/config`。

**Q：不想配 Runner，只想本地练？**

用 **方式一** `./scripts/deploy.sh` 即可。

---

## 学到了什么

- CI/CD = 代码提交后自动 **构建 → 部署**
- GitHub Actions 和 GitLab CI 只是触发器不同，部署逻辑可以共用
- K8s `kubectl set image` 触发滚动更新，版本号变化 = 部署成功的标志
