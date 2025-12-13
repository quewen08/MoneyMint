# 使用Node.js 20作为基础镜像
FROM node:20 as builder

# 设置工作目录
WORKDIR /app

# 复制package.json和package-lock.json
COPY . .

# 安装依赖 并 构建项目
RUN npm install --production && npm run build

FROM node:20-slim as runner

# 设置工作目录
WORKDIR /app

RUN npm install pm2 -g

# 复制构建好的项目文件
COPY --from=builder /app/.output ./.output
COPY --from=builder /app/ecosystem.config.cjs ./

# 暴露端口
EXPOSE 3000

# 启动应用
CMD ["pm2-runtime", "ecosystem.config.cjs"]