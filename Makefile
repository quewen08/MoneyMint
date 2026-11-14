# FamBean 常用命令（make 环境 / CI 用）
#
# 等价入口（不依赖 make）：
#   bash scripts/local.sh <command>
#
# cutover（批 6）后仓库只有一套前端（Vue 3 SPA），Flutter 构建链已清退。

GO     ?= go
SHELL  := bash

.PHONY: help dev build web-build web-dev web-test web-typecheck web-compat check-legacy test test-web check version smoke rehearse image clean

help:
	@echo "make dev           启动后端（托管 frontend/dist）"
	@echo "make build         编译后端二进制到 dist/"
	@echo "make web-build     构建 Vue 前端（Vite → frontend/dist）"
	@echo "make web-dev       启动 Vite 开发服务器（/api 代理到 127.0.0.1:8080）"
	@echo "make web-test      Vue 单元测试（Vitest）"
	@echo "make web-typecheck Vue 类型检查（vue-tsc）"
	@echo "make web-compat    扫描构建产物 es2020 语法 + §14.7 API/CSS 特性（es-check + compat.mjs）"
	@echo "make check-legacy  校验冻结的 /legacy/ 回滚基准（dist/legacy/，不再重建）"
	@echo "make test          后端 vet + 全部测试（严格 bean-check）"
	@echo "make test-web      Vue 单元测试（与 web-test 同义，历史命令名保留）"
	@echo "make check         test + bean-check 自检（CI 同款）"
	@echo "make version       版本号三处一致性校验"
	@echo "make smoke         端到端冒烟（起服务→记账→导出→校验→备份）"
	@echo "make rehearse      迁移 012 副本库预演（VACUUM INTO + 两次迁移 + bean-check）"
	@echo "make image         本地构建 Docker 镜像"

dev:
	LEDGER_WEB_ROOT=frontend/dist $(GO) run ./cmd/server

build:
	mkdir -p dist
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "-s -w" -o dist/server ./cmd/server

# Vue 前端：npm 命令与 scripts/local.sh 的 web-* 一一对应
web-build:
	cd frontend && npm run build

web-dev:
	cd frontend && npm run dev

web-test:
	cd frontend && npm run test

web-typecheck:
	cd frontend && npm run typecheck

web-compat:
	cd frontend && npm run compat

check-legacy:
	bash scripts/check-legacy.sh

test:
	$(GO) vet ./...
	BEAN_CHECK_STRICT=1 $(GO) test ./...

# 历史命令名保留，与 web-test 同义（原为 flutter analyze/test）
test-web:
	cd frontend && npm run test

check: test version
	bash scripts/check-bean.sh

version:
	bash scripts/verify-version.sh

smoke:
	bash scripts/smoke.sh

rehearse:
	bash scripts/rehearsal.sh

image:
	docker build -t famben:dev .

clean:
	rm -rf dist data/smoke
