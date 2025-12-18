# 多阶段构建：前端构建阶段
FROM node:20-alpine as frontend-builder

# 设置工作目录
WORKDIR /app/frontend

# 复制前端依赖文件
COPY frontend/package.json frontend/pnpm-lock.yaml ./

# 安装依赖
RUN npm install pnpm -g && pnpm install --frozen-lockfile

# 复制前端代码
COPY frontend/ .

# 构建前端
RUN pnpm run build

# 后端构建阶段
FROM python:3.11-slim as backend-builder

# 设置工作目录
WORKDIR /app/backend

# 复制后端依赖文件
COPY backend/ ./

# 安装后端依赖
RUN pip install --no-cache-dir -r requirements.txt

# 最终运行阶段
FROM python:3.11-slim

# 设置工作目录
WORKDIR /app

# 安装Node.js用于运行前端
RUN apt-get update && apt-get install -y nodejs npm

# 安装PM2
RUN npm install pm2 -g

# 复制后端代码和依赖
COPY --from=backend-builder /app/backend ./backend
COPY --from=backend-builder /usr/local/lib/python3.11/site-packages /usr/local/lib/python3.11/site-packages

# 复制前端构建产物
COPY --from=frontend-builder /app/frontend/.output ./frontend/.output
COPY --from=frontend-builder /app/frontend/ecosystem.config.cjs ./frontend/

# 复制数据目录（如果存在）
COPY backend/data ./backend/data

# 创建环境变量文件
RUN echo "LEDGER_FILE=/app/backend/data/main.bean" > ./backend/.env

# 暴露端口
EXPOSE 3000

# 启动脚本
RUN echo '#!/bin/bash\ncd /app/backend && python app/main.py &\ncd /app/frontend && pm2-runtime ecosystem.config.cjs' > /app/start.sh && chmod +x /app/start.sh

# 启动应用
CMD ["/app/start.sh"]