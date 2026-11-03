# FamBean 常用命令（Linux / macOS / CI 用；Windows 请用 scripts/local.sh）
#
# 说明：本机（Windows Git Bash）通常没有 make，直接用：
#   bash scripts/local.sh <command>

GO      ?= go
FLUTTER ?= flutter
SHELL   := bash

.PHONY: help dev build web test test-web check version smoke image clean

help:
	@echo "make dev       启动后端（托管 frontend/build/web）"
	@echo "make build     编译后端二进制到 dist/"
	@echo "make web       构建前端产物到 frontend/build/web"
	@echo "make test      后端 vet + 全部测试（严格 bean-check）"
	@echo "make test-web  前端静态分析"
	@echo "make check     test + bean-check 自检（CI 同款）"
	@echo "make version   版本号三处一致性校验"
	@echo "make smoke     端到端冒烟（起服务→记账→导出→校验→备份）"
	@echo "make image     本地构建 Docker 镜像"

dev:
	LEDGER_WEB_ROOT=frontend/build/web $(GO) run ./cmd/server

build:
	mkdir -p dist
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "-s -w" -o dist/server ./cmd/server

web:
	cd frontend && $(FLUTTER) build web --release

test:
	$(GO) vet ./...
	BEAN_CHECK_STRICT=1 $(GO) test ./...

test-web:
	cd frontend && $(FLUTTER) analyze lib

check: test version
	bash scripts/check-bean.sh

version:
	bash scripts/verify-version.sh

smoke:
	bash scripts/smoke.sh

image:
	docker build -t famben:dev .

clean:
	rm -rf dist data/smoke
