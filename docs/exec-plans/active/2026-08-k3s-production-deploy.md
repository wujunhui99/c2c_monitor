# 2026-08 K3s Production Deploy

状态：已完成，待发布

## 背景

`agenticim.xyz` 已运行在单节点 K3s 集群，入口由 Traefik 提供，TLS 由 cert-manager
`letsencrypt-prod` 签发。C2C Monitor 之前的 Ubuntu Compose 部署已经失效，正确生产主机是
`207.57.131.50:9093`。

## 目标

- 在独立 `c2c-monitor` namespace 部署 MySQL、backend 和 frontend
- 通过 `https://c2c.agenticim.xyz` 提供公网访问
- 使用完整 commit SHA 镜像，不使用 `latest`
- 首次部署生成数据库和管理员密钥，仓库不保存真实 secret
- GitHub Actions 等待 rollout、证书和公网健康检查全部通过
- 不修改或重启现有 `agents-im` namespace 资源

## 验证标准

- `make doctor`
- `go test -race ./...`
- 后端和前端镜像构建成功
- `kubectl kustomize deploy/k8s` 渲染成功
- `kubectl -n c2c-monitor get pod,svc,ingress,certificate,pvc` 全部健康
- `https://c2c.agenticim.xyz/healthz` 返回 `200`
- `https://c2c.agenticim.xyz/readyz` 返回 `200`
- `https://c2c.agenticim.xyz/api/meta` 返回当前版本
