#!/bin/bash
# 强制拉取最新代码并重启 GSCoWork

APP_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$APP_DIR"

echo "=== 停止 GSCoWork ==="
./gscowork stop

echo "=== 拉取最新代码 ==="
git fetch --all
git reset --hard origin/main

echo "=== 编译 ==="
GOOS=linux GOARCH=amd64 go build -o gscowork . || { echo "编译失败"; exit 1; }
chmod +x gscowork update_and_run.sh

echo "=== 启动 GSCoWork ==="
./gscowork start
