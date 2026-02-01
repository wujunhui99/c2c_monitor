#!/bin/bash

# 前端启动脚本
PORT=8080

echo "🚀 Attempting to start Frontend on port $PORT..."

# 1. 尝试使用 live-server (支持热更新)
if command -v live-server >/dev/null 2>&1; then
    echo "Using: live-server"
    live-server --port=$PORT
# 2. 尝试使用 npx 运行 live-server
elif command -v npx >/dev/null 2>&1; then
    echo "Using: npx live-server"
    npx live-server --port=$PORT
# 3. 尝试使用 Python 3
elif command -v python3 >/dev/null 2>&1; then
    echo "Using: python3 http.server"
    python3 -m http.server $PORT
# 4. 尝试使用 Python 2
elif command -v python >/dev/null 2>&1; then
    echo "Using: python SimpleHTTPServer"
    python -m SimpleHTTPServer $PORT
else
    echo "❌ Error: No suitable web server found. Please install Node.js or Python."
    exit 1
fi
