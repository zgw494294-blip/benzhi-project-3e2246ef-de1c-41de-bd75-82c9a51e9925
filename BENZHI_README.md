# BENZHI_README

基于 Go 实现的口述语料脱敏放行台 HTTP API 项目，一款后端服务，用于支持口述语料脱敏放行台的核心业务流程。

## 项目说明
- 项目：benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925
- 项目用途：用于支持口述语料脱敏放行台的核心业务流程。
- Go 工具链：`golang:1.23`
- 前端工具链：无

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925-arm64 linux/arm64
docker run -it benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck`
