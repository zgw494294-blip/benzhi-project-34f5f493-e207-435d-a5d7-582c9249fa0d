# BENZHI_README

基于 Go 实现的临时活动用电送电许可工作台 Web 项目，一款后端服务，完整实现临时活动用电从申请建档、负荷方案、现场检查、整改复检、安全复核到冻结并签发送电许可的浏览器工作台，使用带校验链的本地事件日志和原子快照持久化。

## 项目说明
- 项目：benzhi-project-34f5f493-e207-435d-a5d7-582c9249fa0d
- 项目用途：完整实现临时活动用电从申请建档、负荷方案、现场检查、整改复检、安全复核到冻结并签发送电许可的浏览器工作台，使用带校验链的本地事件日志和原子快照持久化。
- Go 工具链：`golang:1.22`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/powerpermit -addr=127.0.0.1:19081 -selfcheck
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-34f5f493-e207-435d-a5d7-582c9249fa0d-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-34f5f493-e207-435d-a5d7-582c9249fa0d-arm64 linux/arm64
docker run -it benzhi-project-34f5f493-e207-435d-a5d7-582c9249fa0d-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/powerpermit -addr=127.0.0.1:19081 -selfcheck`
