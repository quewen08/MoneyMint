# FamBean M0 + M1 开发任务书

> 版本：v1.0 ｜ 日期：2026-11-03 ｜ 角色：全栈开发
> 上游输入：`docs/m0-m1-requirements.md`（产品侧拆解）+ `docs/product-review-2026-11.md` v2.1（路线图）
> 性质：可执行开发任务书。含技术决策、文件级改动清单、代码骨架、验收命令。研发照此可直接开工。

---

## 一、开工前：产品需求的可疑点与技术反查

产品侧的 10 个原子任务整体成立，但开发视角反查到 **5 个必须在动手前钉死的点**（否则做到一半会返工）：

| # | 产品需求原文 | 开发反查结论 | 处理 |
|---|---|---|---|
| 1 | 「Go 服务托管前端静态产物」 | 现有 `authMiddleware` **对所有非白名单路径强制校验 token**，不加改造访问 `/` 会直接 401；且 `newRouter` 只注册了 `/api/*`，未注册 `catch-all` | M0-b 必须先改中间件放行规则，再挂静态 handler。**这是最容易踩的坑** |
| 2 | 「一致性快照用 `VACUUM INTO`」 | `db.go:31` 设了 `SetMaxOpenConns(1)`，**全库只有一条连接**。`VACUUM INTO` 会独占这条连接，期间所有 API 阻塞 | 家庭数据量下耗时应在毫秒级，可接受；但必须加**互斥锁 + 超时保护 + 实测记录耗时**，且备份失败不能影响业务请求。**`VACUUM INTO` 可行性已由开发侧实测通过，见 §1.1** |
| 3 | 「系统管理员 `users.is_admin`」 | 迁移框架为「数组追加 + `columnExists` 幂等」模式，可平滑加 `008`。但**旧库无 admin 时需要自动补选**，且要考虑「首个注册用户已被移除」的边界 | M1-a 需明确 3 种场景的补选规则（见任务 M1-a） |
| 4 | 「备份下载要做路径穿越防护」 | 现有 handler 全部走 `readJSON` + 路径参数，无文件下载先例；`writeJSON` 不适用于二进制流 | M1-d 需新写下载 handler（`io.Copy` + `Content-Disposition`），不能复用现有辅助函数 |
| 5 | 「版本号三处一致由 CI 校验」 | 现有无任何 CI 脚本，`pubspec.yaml` 的 `version: 1.0.0+1` 与 CHANGELOG `0.4.0` 已漂移，**首次校验必然失败** | M0-d 需先做一次人工对齐（建议统一到 `0.5.0`），否则 CI 一上线就红 |

**除此之外的隐性约束（已在现有代码里固化，不要打破）**：

- 分层铁律：handler 薄（只做 JSON/参数/状态码）→ service（业务）→ repository（SQL）。**SQL 不许出现在 service 层**。
- 错误处理：`domain` 哨兵错误 + `handler.statusOf` 映射，新增错误要挂到既有哨兵上（`ErrNotFound` / `ErrConflict` / `ErrForbidden`），不要新造 HTTP 状态码映射。
- 迁移：只在 `internal/db/migrate.go` 的 `migrations` 数组追加，禁止改已有迁移函数体。
- 金额：定点十进制字符串，**禁止 float**。本次 M0/M1 不涉及金额计算，但备份校验比对行数时不要顺带算金额。
- 前端：页面读 `AppScope.of(context)` **必须在 `build`/`didChangeDependencies`**，禁止放 `initState`（会红屏）。

### 1.1 ✅ 已实测：`VACUUM INTO` 可行性（原计划列为前置任务，已提前验证完毕）

开发侧用 `modernc.org/sqlite`（项目当前版本 v1.56.0）跑了实测，**结论全部有利，可以直接按方案实施**：

| 验证项 | 结果 | 对实现的影响 |
|---|---|---|
| `VACUUM INTO '<字面量路径>'` | ✅ 成功 | 可用 |
| `VACUUM INTO ?`（**参数化**） | ✅ **成功** | **不必字符串拼接**，直接用 `ExecContext(ctx, "VACUUM INTO ?", path)`，无注入风险 |
| 目标文件已存在时 | ❌ **报错** `SQL logic error: output file already exists` | 必须保证目标文件名唯一、不存在（用 `YYYYMMDD-HHmmss` + 校验） |
| 快照数据完整性 | ✅ 500/500 行全部在，`integrity_check=ok` | 方案成立 |
| **直接复制主文件（对照实验）** | ❌ **0 行** | 铁证，见下 |

**对照实验的硬数据**（这是本次最关键的发现）：

```
WAL_SIZE_BYTES  = 2,080,632   (约 2 MB)
MAIN_DB_BYTES   =     4,096   (4 KB，仅剩库头与 schema)

VACUUM INTO 快照 → rows=500, integrity=ok     ✅ 正确做法
直接复制主文件   → rows=0                     ❌ 数据 100% 丢失
```

**结论**：在 WAL 模式下，未 checkpoint 的数据几乎全部驻留在 `-wal` 文件里（本例主文件只剩 4 KB）。**直接 `cp ledger.db` 会得到一份「能打开、有表结构、但一行数据都没有」的空壳库**——而且它不会报错，会安静地成为一个假备份。这比备份失败更危险。

> 因此 M1-b 的核心测试用例必须是「写入 N 条 → 不做 checkpoint → 快照 → 断言快照内 N 条齐全」，而不是「快照文件存在且能打开」。

---

## 二、技术决策（Phase 1）

| # | 决策 | 理由（一句话） |
|---|---|---|
| D1 | **静态托管独立成 `internal/server/static.go`**，不放 handler 包 | handler 是 JSON 适配层，塞二进制/文件服务会破坏分层；静态托管本质是路由层职责 |
| D2 | **`authMiddleware` 改为「只对 `/api/` 前缀做鉴权」** | 一条规则同时解决：静态资源放行 + 未来所有非 API 路径天然公开，比维护白名单可靠 |
| D3 | **备份独立成 `internal/backup` 包**（engine + scheduler），service 只做编排 | 涉及文件系统、定时器、互斥锁、完整性校验，塞进 service 会让业务层变重且难测 |
| D4 | **快照用 `VACUUM INTO`，并保留 fallback** | 一致性最强、代码最少；但 `modernc.org/sqlite` 对此语法的支持需实测，失败则降级为「checkpoint + 复制 + integrity_check」 |
| D5 | **admin 权限用独立中间件 `requireAdmin`**，串在 `authMiddleware` 之后 | 职责单一，后续 M3 的会话管理、公开前的注册控制都能复用它 |
| D6 | **版本源 = git tag，校验脚本用纯 bash** | CI 环境零额外依赖，且 bash 在本地 Git Bash 与 ubuntu-latest 上行为一致 |
| D7 | **单镜像**（Flutter builder → Go builder → alpine runtime） | 家庭 NAS 部署组件越少越好；顺带补齐「一个端口可用」的缺口 |
| D8 | **错误处理沿用领域哨兵**，备份失败包装成 `fmt.Errorf("...: %w", err)` 供日志追溯，对外统一返回 `{"error": "..."}` | 与现有约定一致，不引入新错误体系 |

---

## 三、代码库锚点（开工必看）

| 关注点 | 文件 | 现状要点 |
|---|---|---|
| 路由注册 | `internal/server/router.go` | `newRouter(h, svc)` 只注册 `/api/*`；`methodSwitch` 做方法分发；外层 `cors(authMiddleware(...))` |
| 鉴权中间件 | `internal/server/middleware.go` | **白名单硬编码**（`/api/health`、register、login），其余一律要求 token；`writeJSON` 在此包内已有一份 |
| 组合根 | `internal/server/server.go` | `config → db.Open → repository.New → service.New → handler.New → newRouter` |
| 配置 | `internal/config/config.go` | `Load()` + `envOr`，**无校验、无快速失败**（本次要补） |
| 迁移 | `internal/db/migrate.go` | `migrations` 数组（001~007）；`columnExists` 可复用 |
| 分层样板 | `internal/handler/account.go` + `internal/service/account.go` + `internal/repository/account.go` | 照着这三个文件的写法新增 |
| 导出测试 | `internal/export/beancount_test.go:116-137` | 现有逻辑：依次试 `python -m beancount.scripts.check` 与 `bean-check`，**找不到就 `t.Logf` + skip** |
| 前端 API 客户端 | `frontend/lib/core/api/ledger_api.dart` | 静态 token 存 localStorage；`_ledgerId` 控制 `X-Ledger-Id`；`kDefaultApiBase` 硬编码 |
| 前端设置页 | `frontend/lib/screens/settings/settings_screen.dart` | `AppCard` + `SectionTitle` 组织；`_DefaultAccountsCard` 可作新增卡片的样板 |
| 忽略规则 | `.dockerignore` | **已忽略 `frontend/build/`**（正确，Docker 里从源码重建）；已忽略 `data/`、`*.db*` |

---

## 四、任务清单

### M0-a · `bean-check` 严格化 + 双路径校验

**目标**：导出兼容性校验不允许静默跳过；单文件与多文件目录两条导出路径都要实校。

**改动文件**

| 操作 | 文件 |
|---|---|
| 修改 | `internal/export/beancount_test.go` |
| 新增 | `internal/export/archive_test.go`（多文件目录导出校验） |
| 新增 | `scripts/check-bean.sh`（CI 与本地共用的校验入口） |

**实现要点**

1. 抽公共函数 `runBeanCheck(t *testing.T, path string)`：
   - 候选命令依次为 `python -m beancount.scripts.check <path>`、`bean-check <path>`（保持现状探测顺序）。
   - 读环境变量 `BEAN_CHECK_STRICT`：为 `1`/`true` 时，**全部候选都不可用 → `t.Fatalf` 并输出安装指引**；否则 `t.Skip` 但打印醒目警告（`⚠️ 未校验`）。
   - 校验失败（退出码非 0）→ `t.Fatalf` 并附完整 stdout/stderr。
2. `TestExportTags` 改为调用 `runBeanCheck`，删除内联的探测逻辑。
3. 新增 `TestExportArchiveBeanCheck`：调 `export` 包的多文件目录导出 → 解压到 `t.TempDir()` → 对 `main.bean` 跑 `runBeanCheck`。**注意 `include` 路径是相对路径，必须在解压根目录下执行 `bean-check`**。
4. `scripts/check-bean.sh`：检测 `beancount` 是否可用，不可用则 `exit 1` 并打印 `pip install beancount`；供 CI 与 `make check` 调用。

**关键骨架**

```go
func runBeanCheck(t *testing.T, path string) {
    t.Helper()
    strict := os.Getenv("BEAN_CHECK_STRICT") == "1" || os.Getenv("BEAN_CHECK_STRICT") == "true"
    cands := [][]string{
        {"python", "-m", "beancount.scripts.check", path},
        {"python3", "-m", "beancount.scripts.check", path},
        {"bean-check", path},
    }
    var lastErr error
    for _, c := range cands {
        cmd := exec.Command(c[0], c[1:]...)
        out, err := cmd.CombinedOutput()
        if errors.Is(err, exec.ErrNotFound) { lastErr = err; continue }
        if err != nil {
            t.Fatalf("bean-check 校验失败(%v): %v\n输出:\n%s", c, err, out)
        }
        t.Logf("bean-check 通过（%v）", c)
        return
    }
    if strict {
        t.Fatalf("严格模式：未找到 bean-check，请安装 beancount（pip install beancount）。底层错误: %v", lastErr)
    }
    t.Logf("⚠️ bean-check 不可用，跳过校验（非严格模式）")
}
```

**验收**

```bash
export PATH="$PATH:/d/Environment/go/bin"
BEAN_CHECK_STRICT=1 go test ./internal/export/ -run 'TestExport' -v   # 必须实际执行校验并 PASS
mv .venv .venv_bak && BEAN_CHECK_STRICT=1 go test ./internal/export/ -run 'TestExportTags'
# 期望：FAIL，且错误信息含「严格模式：未找到 bean-check」
mv .venv_bak .venv
```

**风险**：`TestExportArchiveBeanCheck` 解压后 `main.bean` 内含 `include "accounts/..."`，若在错误工作目录执行会报「文件不存在」——必须在解压根目录用 `cmd.Dir` 设定。

---

### M0-b · Go 托管前端静态产物 + 中间件放行

**目标**：单端口可用。浏览器访问 `/` 得到 Flutter 应用，前端路由（如 `/transactions`）能 SPA 回退，`/api/*` 行为不变。

**改动文件**

| 操作 | 文件 |
|---|---|
| 新增 | `internal/server/static.go` |
| 修改 | `internal/server/middleware.go`（**核心：鉴权范围收窄到 `/api/`**） |
| 修改 | `internal/server/router.go`（挂载 catch-all） |
| 修改 | `internal/config/config.go`（新增 `WebRoot`） |
| 修改 | `internal/server/server.go`（装配 WebRoot） |

**实现要点**

1. `config.Config` 增 `WebRoot string`，`Load()` 读 `LEDGER_WEB_ROOT`（默认 `./web`）。
2. **`authMiddleware` 改造**（最关键）：

```go
func authMiddleware(svc *service.Service, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 非 /api/ 前缀：静态资源与 SPA 入口，无需鉴权（前端自行控制登录态）
        if !strings.HasPrefix(r.URL.Path, "/api/") {
            next.ServeHTTP(w, r)
            return
        }
        // ...以下保持原逻辑（health/register/login 白名单、token 校验、账本角色）...
    })
}
```

3. `static.go` 实现 `spaHandler(root string) http.HandlerFunc`：
   - 路径清洗：`path.Clean("/" + r.URL.Path)`，**拒绝含 `..` 的请求**。
   - 命中规则：`root + cleanPath` 存在且是文件 → 直接 serve；否则回退 `root/index.html`。
   - **关闭目录列表**：不能直接用 `http.FileServer`（会列目录），必须自己 `os.Open` + `http.ServeContent`，或用 `http.FileServer(http.Dir(root))` 但先自行判定文件存在性。
   - 缓存头：
     - `index.html`、`flutter_bootstrap.js`、`flutter_service_worker.js`、`manifest.json` → `Cache-Control: no-cache`
     - `/assets/`、`/canvaskit/`、含 hash 的文件 → `Cache-Control: public, max-age=31536000, immutable`
     - 其余静态 → `Cache-Control: public, max-age=3600`
   - 加基础安全头：`X-Content-Type-Options: nosniff`。
4. `router.go` 末尾挂载：

```go
// 静态资源 + SPA（必须在 /api/ 路由之后注册；ServeMux 按最长前缀匹配，"/" 兜底）
mux.HandleFunc("/", spaHandler(cfg.WebRoot))
```

> 注意：`newRouter` 目前签名是 `newRouter(h, svc)`，需改为 `newRouter(h, svc, cfg)` 或把 `webRoot` 传进来。**改签名要同步 `server.go` 的调用点。**

5. 目录不存在时：**不报错、不退出**，启动日志打印 `未托管前端（LEDGER_WEB_ROOT=... 不存在）`，`/api/health` 正常。

**验收**

```bash
cd frontend && flutter build web --release
cd .. && LEDGER_WEB_ROOT=frontend/build/web go run ./cmd/server &
curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8080/            # 200
curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8080/transactions # 200 且返回 index.html
curl -sI http://127.0.0.1:8080/ | grep -i cache-control                     # no-cache
curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8080/api/accounts  # 401（鉴权仍生效）
curl -s -o /dev/null -w "%{http_code}\n" "http://127.0.0.1:8080/../go.mod"   # 400/404，不泄露
```

**风险**：
- `ServeMux` 的 `/` 会兜住所有未匹配路径，**必须确认 `/api/*` 的精确路由优先**（Go 1.22+ 的 ServeMux 支持模式匹配，最长匹配优先，但要在实测中确认 `/api/unknown` 不会落到静态 handler 返回 HTML）。建议：在 `spaHandler` 内对 `/api/` 前缀显式返回 404 JSON。
- Flutter Web 首次加载需 `flutter_bootstrap.js` 与 `canvaskit` 相对路径正确，`<base href="/">` 已在 `web/index.html` 中，不要改。

---

### M0-c · 多阶段 Dockerfile

**目标**：一条命令产出可在 amd64/arm64 上直接跑的单镜像。

**改动文件**：新增 `Dockerfile`；修改 `.dockerignore`（补 `frontend/test/`、`*.md` 可选）

**实现要点**

1. 三段构建：

```dockerfile
# ---------- Stage 1: Flutter Web ----------
FROM ghcr.io/cirruslabs/flutter:3.35.1 AS web-builder
WORKDIR /src/frontend
COPY frontend/pubspec.yaml frontend/pubspec.lock ./
RUN flutter pub get --enforce-lockfile
COPY frontend/ ./
RUN flutter build web --release --no-tree-shake-icons

# ---------- Stage 2: Go ----------
FROM golang:1.25-alpine AS go-builder
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG BUILD_DATE BUILD_HASH GIT_VERSION
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X main.buildDate=${BUILD_DATE} -X main.buildHash=${BUILD_HASH} -X main.gitVersion=${GIT_VERSION}" \
    -o /out/server ./cmd/server

# ---------- Stage 3: Runtime ----------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
WORKDIR /app
COPY --from=go-builder /out/server /app/server
COPY --from=web-builder /src/frontend/build/web /app/web
COPY schema.sql /app/schema.sql
RUN mkdir -p /app/data && chown -R app:app /app
USER app
ENV LEDGER_DB=/app/data/ledger.db \
    LEDGER_SCHEMA=/app/schema.sql \
    LEDGER_WEB_ROOT=/app/web \
    LISTEN=:8080 \
    TZ=Asia/Shanghai
EXPOSE 8080
VOLUME ["/app/data"]
ENTRYPOINT ["/app/server"]
```

2. `main.go` 补三个变量（供 `-X` 注入 + 启动日志打印）：

```go
var (
    buildDate  = "unknown"
    buildHash  = "unknown"
    gitVersion = "unknown"
)
// main() 内：log.Printf("version=%s hash=%s build=%s", gitVersion, buildHash, buildDate)
```

3. **`.dockerignore` 必须确认** `frontend/build/`、`.dart_tool/` 被忽略（现状已忽略，正确）；确保 `pubspec.lock` **不被**忽略（现状未被忽略，正确）。

**验收**

```bash
docker build -t fambean:dev .
docker run --rm -d -p 8080:8080 -v "$PWD/dsmoke:/app/data" --name fb fambean:dev
docker exec fb id                                  # uid=10001，非 root
curl -s http://127.0.0.1:8080/api/health           # {"status":"ok"}
curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8080/  # 200（前端已托管）
docker logs fb | grep version                       # 版本信息可见
docker restart fb && curl -s http://127.0.0.1:8080/api/health  # 数据仍在
docker image ls fambean:dev --format "{{.Size}}"    # 目标 < 120MB，超了要查原因
```

**风险**：
- `ghcr.io/cirruslabs/flutter` 版本 tag 需确认存在（3.35.1）。若拉不到，退 `flutter:stable` 但要在文档记录实际版本。
- Flutter 首次构建在 CI 上可能 10+ 分钟，**必须配 buildx 缓存**（见 M0-e）。
- 镜像体积主要来自 `build/web`（canvaskit 约 6~8MB）+ Go 二进制（约 20MB）。若超 120MB，先查是否误拷了 `build/web/canvaskit` 的多份变体。

---

### M0-d · 版本号治理与一致性校验

**目标**：tag、CHANGELOG、pubspec 三处锁死，任意一处漂移 CI 立即失败。

**改动文件**：新增 `scripts/verify-version.sh`；修改 `CHANGELOG.md`（补 0.5.0 占位或确认当前版本）、`frontend/pubspec.yaml`

**实现要点**

1. **先做一次人工对齐**（否则校验一上线就红）：确认将要发布的版本号（建议 `0.5.0`），同步改：
   - `CHANGELOG.md` 顶部新增 `## [0.5.0] - YYYY-MM-DD`（内容可先写 `### Added` 占位）
   - `frontend/pubspec.yaml` → `version: 0.5.0+1`
2. `scripts/verify-version.sh` 逻辑：
   - 入参：`$1` = tag（如 `v0.5.0`）；无入参时从 `git describe --tags --abbrev=0` 取。
   - 剥离前缀 `v` 得 `VER`。
   - 从 `CHANGELOG.md` 提取首个 `## \[(\d+\.\d+\.\d+)\]`。
   - 从 `pubspec.yaml` 提取 `^version:\s*([0-9.]+)`（去掉 `+build`）。
   - 三者不等 → 打印**具体差异表**并 `exit 1`。
3. 脚本要能在 Windows Git Bash 与 ubuntu-latest 同时跑（只用 grep/sed/awk，不依赖 GNU 特有参数）。

**验收**

```bash
bash scripts/verify-version.sh v0.5.0        # 三处一致 → exit 0
sed -i 's/0.5.0/0.4.9/' frontend/pubspec.yaml
bash scripts/verify-version.sh v0.5.0        # exit 1，且打印 pubspec=0.4.9 vs tag=0.5.0
git checkout frontend/pubspec.yaml
```

---

### M0-e · CI（GitHub Actions）+ compose + Makefile

**目标**：推送 `v*` tag 自动构建双架构镜像推 Docker Hub；本地一条命令跑全量检查。

**新增文件**：`.github/workflows/docker-image.yml`、`docker-compose.yml`、`Makefile`

**workflow 要点**

```yaml
name: build-and-push
on:
  push:
    tags: ['v*']
  workflow_dispatch:

jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.25' }
      - uses: actions/setup-python@v5
        with: { python-version: '3.11' }
      - run: pip install beancount
      - run: bash scripts/verify-version.sh "${GITHUB_REF_NAME}"
      - run: go vet ./...
      - run: go test ./... -v
        env: { BEAN_CHECK_STRICT: '1' }     # 关键：严格模式
      - run: bash scripts/check-bean.sh

  build:
    needs: verify            # 校验不过不推镜像
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-qemu-action@v3
      - uses: docker/setup-buildx-action@v3
      - uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKER_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}
      - id: ver
        run: echo "v=${GITHUB_REF_NAME#v}" >> $GITHUB_OUTPUT
      - uses: docker/build-push-action@v6
        with:
          push: true
          platforms: linux/amd64,linux/arm64
          build-args: |
            BUILD_DATE=${{ github.event.repository.updated_at }}
            BUILD_HASH=${{ github.sha }}
            GIT_VERSION=${{ steps.ver.outputs.v }}
          tags: |
            ${{ secrets.DOCKER_USERNAME }}/famben:${{ steps.ver.outputs.v }}
            ${{ secrets.DOCKER_USERNAME }}/famben:latest
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

> ⚠️ 镜像名 `famben` 是占位，**待老板定名**（见 §7 Q-2）。定名后需同步 `compose` 与 workflow 两处。

**Makefile 要点**

```makefile
.PHONY: dev web test check smoke image
dev:    ; LEDGER_WEB_ROOT=frontend/build/web go run ./cmd/server
web:    ; cd frontend && flutter build web --release
test:   ; go vet ./... && go test ./... && cd frontend && flutter analyze lib
check:  ; test && bash scripts/check-bean.sh
image:  ; docker build -t famben:dev .
smoke:  ; bash scripts/smoke.sh
```

**docker-compose.yml 要点**

```yaml
services:
  famben:
    image: ${DOCKER_USERNAME}/famben:latest
    container_name: famben
    restart: unless-stopped
    ports: ["8080:8080"]
    environment:
      TZ: Asia/Shanghai
    volumes:
      - ./data:/app/data        # 数据 + 备份（backups/ 会在同目录生成）
```

**验收**

- 手动 `workflow_dispatch` 跑通一次（无 tag 时版本校验会失败，需临时输入或先打 tag）。
- Docker Hub 上能看到 `latest` 与版本 tag，且 `docker buildx imagetools inspect` 显示 `amd64` + `arm64`。
- 本地 `make check` 全绿。

---

### M1-a · 系统管理员角色（迁移 008）

**目标**：引入全局管理员，用于全库级备份操作。

**改动文件**

| 操作 | 文件 |
|---|---|
| 修改 | `internal/db/migrate.go`（追加 `008_user_admin`） |
| 修改 | `internal/domain/models.go`（`User` 加 `IsAdmin bool`） |
| 修改 | `internal/repository/user.go`（查询/设置 admin） |
| 修改 | `internal/handler/auth.go`（`me` 返回 `is_admin`） |
| 修改 | `internal/server/middleware.go`（新增 `requireAdmin`） |
| 修改 | `internal/service/auth.go`（注册时判定首个用户） |

**实现要点**

1. 迁移 `008_user_admin`：
   ```go
   func migrate008(conn *sql.DB) error {
       if ok, err := columnExists(conn, "users", "is_admin"); err != nil {
           return err
       } else if !ok {
           if _, err := conn.Exec(`ALTER TABLE users ADD COLUMN is_admin INTEGER NOT NULL DEFAULT 0`); err != nil {
               return fmt.Errorf("add users.is_admin: %w", err)
           }
       }
       // 旧库补选：若当前无 admin，将 id 最小且未删除的用户置为 admin
       var n int
       if err := conn.QueryRow(`SELECT count(*) FROM users WHERE is_admin=1`).Scan(&n); err != nil {
           return err
       }
       if n == 0 {
           if _, err := conn.Exec(
               `UPDATE users SET is_admin=1 WHERE id=(SELECT min(id) FROM users WHERE status IS NULL OR status<>'deleted')`,
           ); err != nil {
               return fmt.Errorf("auto assign admin: %w", err)
           }
       }
       return nil
   }
   ```
   > **注意**：`users` 表是否真有 `status` 列需按 `schema.sql` 核对后再写 WHERE 条件，不要照抄。
2. 注册时：`CreateUser` 后若「该系统此前无任何用户」→ 置 `is_admin=1`。判断要在**同一事务**内，避免并发双注册都成 admin。
3. `me` 响应增 `"is_admin": bool`。
4. `requireAdmin` 中间件：从 context 取 `uid` → 查 `is_admin` → 否返回 `403 {"error":"需要管理员权限"}`。**必须在 `authMiddleware` 之后调用**（依赖 context 里的 uid）。
5. 旧库若 `users` 为空（全新库）→ 不置任何 admin，等首个注册用户。

**验收**

```bash
# 全新库
rm -rf data/test_admin.db* && LEDGER_DB=data/test_admin.db go run ./cmd/server &
curl -s -X POST :8080/api/auth/register -d '{"username":"a","password":"abcdef123"}' ...
curl -s :8080/api/auth/me -H "Authorization: Bearer $T"    # is_admin: true
# 第二个用户
curl ... register b; curl ... /me                          # is_admin: false
# 旧库升级
cp data/ledger.db data/old.db && 启动                                       # 应自动补选 admin
```

---

### M1-b · 快照引擎

**目标**：生成一致性快照，且保证「将来一定能还原」。

**改动文件**：新增 `internal/backup/engine.go`、`internal/backup/engine_test.go`；修改 `internal/config/config.go`

**实现要点**

1. 结构：
   ```go
   type Engine struct {
       db       *sql.DB
       dbPath   string
       dir      string
       mu       sync.Mutex   // 同一时刻只允许一个备份
       logf     func(format string, args ...any)
   }
   type Result struct {
       Name string; Path string; Size int64; SHA256 string
       Trigger string    // auto | manual
       Stats   map[string]int64  // accounts/transactions/ledgers/users 行数
       StartedAt, FinishedAt time.Time
   }
   ```
2. 快照步骤（顺序不能乱）：
   - `mu.Lock()`（拿不到锁 → 立即返回 `ErrBackupBusy`，不排队）
   - 目标文件名 `ledger-{YYYYMMDD-HHmmss}.db`，**确认文件不存在**（`VACUUM INTO` 目标已存在会直接报错，已实测）
   - `conn.ExecContext(ctx, "VACUUM INTO ?", targetPath)`（**参数化已实测可用，禁止字符串拼接**）
   - 打开快照库：`PRAGMA integrity_check` 必须为 `ok`
   - 行数比对：`accounts` / `transactions` / `ledgers` / `users` 四表 count 与原库一致（**在同一快照时刻值可能漂移，因有并发写** → 允许 `快照 <= 原库`，但**不允许快照 > 原库**，且差额要在日志记录）
   - 计算 SHA256、写 `manifest.json` 条目
   - 生成同名 `.beancount`（复用现有导出器；**失败不算备份失败**，只记警告，因为 db 快照才是主产物）
   - 失败路径：删除残缺文件，返回 error
   - `mu.Unlock()`（`defer`）
3. **超时保护**：`VACUUM INTO` 包在 `context.WithTimeout(30s)` 里（用 `ExecContext`），超时则失败并记录。原因：`MaxOpenConns(1)` 下它独占连接，卡住等于全站不可用。
4. 耗时记录：`t.Logf` 与日志都要打印「快照耗时 XXX ms、体积 XXX KB」，**这是后续评估是否需要优化的唯一依据**。
5. （备选路径，正常情况下不需要）若未来更换 SQLite 驱动或其版本降级导致 `VACUUM INTO` 不可用，fallback 为：`PRAGMA wal_checkpoint(TRUNCATE)` → 复制 db 文件 → `integrity_check`。fallback 必须在日志中**明确标记**，便于排查时知道走的是降级路径。**当前实测 `VACUUM INTO` 可用，不要提前实现 fallback**（YAGNI）。

**关键测试（对应需求 M1-2 的核心验收）**

```go
func TestSnapshotIncludesUncheckpointedWAL(t *testing.T) {
    // 1. 建库 + WAL 模式
    // 2. 写入 N 条交易，不做 checkpoint
    // 3. 确认 -wal 文件确实非空且体积 > 0
    // 4. 调 Engine.Snapshot()
    // 5. 打开快照库，断言交易数 == N（若用 cp 会失败，此测试即护栏）
    // 6. 断言 integrity_check == ok
}
```

**验收**

```bash
go test ./internal/backup/ -run TestSnapshot -v
# 手工：造数据 → 触发快照 → sqlite3 打开快照验证行数与 integrity
```

**风险**：
- ~~`modernc.org/sqlite` 对 `VACUUM INTO` 的支持未实测~~ → **已实测通过（含参数化），风险消除**，见 §1.1。
- 目标文件重名：同一秒内触发两次会撞名 → 由 `mu` 互斥锁保证串行，且秒级精度下不会重复；但仍需在 `VACUUM INTO` 前做 `os.Stat` 检查并兜底。
- 行数比对可能因并发写产生「快照 < 原库」的正常差额，**不要把这种情况判为失败**，否则会产生误报。

---

### M1-c · 定时调度 + 保留清理

**目标**：每日自动备份、启动补跑、按策略清理。

**改动文件**：新增 `internal/backup/scheduler.go`、`internal/backup/retention.go`、`internal/backup/retention_test.go`

**实现要点**

1. `Scheduler`：持有 `Engine` + 配置，提供 `Start(ctx)` / `Stop()`。
2. 启动补跑：读 `manifest.json` 最近一条成功记录，若 `now - lastSuccess > 24h`（或配置间隔）→ 延迟 `rand(0~60s)` 后触发一次。
3. 定时：计算「下一个 `BACKUP_HOUR` 点」的 `time.Duration`，用 `time.NewTimer` 而非 `Ticker`（避免跨天漂移）。
4. 优雅停机：`Stop()` 关闭 channel，`main.go` 里监听 `SIGINT/SIGTERM` → `srv.Shutdown` + `scheduler.Stop()`。
   > 现状 `main.go` 用 `http.ListenAndServe`，**没有优雅停机**。M1-c 顺带补上（用 `http.Server` + `Shutdown(ctx)`）。
5. 保留策略（`retention.go`）：
   - 读 manifest → 按时间倒序
   - 保留：最近 `KEEP_DAILY` 份（任意日） ∪ 每月首日各 1 份（最近 `KEEP_MONTHLY` 个月） ∪ 全部 `manual` 备份
   - 待删 = 全集 − 保留集
   - 删除时**只删 manifest 里登记过的文件名**，且校验文件名匹配 `^ledger-\d{8}-\d{6}\.(db|beancount)$`，不匹配则跳过并记警告
   - 清理只在**新备份成功后**调用
6. 配置项（`config.go`）：`BackupEnabled`（默认 true）、`BackupDir`、`BackupHour`、`BackupKeepDaily`、`BackupKeepMonthly`。

**验收**

```bash
# 用短间隔验证补跑与定时（BackupHour 设为当前小时+1分钟不可行，改为直接单测）
go test ./internal/backup/ -run 'TestRetention|TestScheduler' -v
# 保留策略单测：构造 20 份假 manifest → 断言删 6 留 14，且 manual 不被删
```

---

### M1-d · 备份 API + 安全防护

**目标**：列表 / 下载 / 手动触发三个端点，路径穿越零容忍。

**改动文件**：新增 `internal/handler/backup.go`；修改 `internal/server/router.go`

**接口契约**

| 端点 | 方法 | 权限 | 响应 |
|---|---|---|---|
| `/api/backup/list` | GET | admin | `{"items":[{"name","size","sha256","created_at","trigger","stats":{...}}]}` |
| `/api/backup/download?name=` | GET | admin | 二进制流 + `Content-Disposition: attachment` |
| `/api/backup/run` | POST | admin | `{"name","size","sha256","duration_ms"}` |

**实现要点**

1. 路由注册（注意与静态 handler 的 `/` 不冲突，`/api/` 优先）：
   ```go
   mux.HandleFunc("/api/backup/list", methodSwitch(map[string]http.HandlerFunc{http.MethodGet: h.HandleBackupList}))
   mux.HandleFunc("/api/backup/download", methodSwitch(map[string]http.HandlerFunc{http.MethodGet: h.HandleBackupDownload}))
   mux.HandleFunc("/api/backup/run", methodSwitch(map[string]http.HandlerFunc{http.MethodPost: h.HandleBackupRun}))
   ```
2. **路径穿越防护（最高优先级）**：
   ```go
   name := r.URL.Query().Get("name")
   base := filepath.Base(name)                       // 剥离任何目录成分
   if base != name || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
       writeJSON(w, 400, map[string]string{"error": "非法文件名"}); return
   }
   if !regexp.MustCompile(`^ledger-\d{8}-\d{6}\.(db|beancount)$`).MatchString(base) {
       writeJSON(w, 400, map[string]string{"error": "非法文件名"}); return
   }
   full := filepath.Join(backupDir, base)
   // 二次确认：Join 后仍在 backupDir 下
   if !strings.HasPrefix(filepath.Clean(full), filepath.Clean(backupDir)+string(os.PathSeparator)) {
       writeJSON(w, 400, map[string]string{"error": "非法路径"}); return
   }
   ```
3. 下载用 `os.Open` + `http.ServeContent`（自动处理 Range 与 Content-Length），不读进内存。
4. `requireAdmin` 串在这三个路由上。**实现方式**：由于现有中间件是全局包一层，admin 校验建议放在 handler 内部第一行（`if !svc.IsAdmin(uid) {...}`）而非再造一个中间件——更简单，且与现有 `newRouter` 结构兼容。
   > 决策：**handler 内校验**优于新增中间件。理由是现有 `newRouter` 里没有「按路由挂中间件」的能力，改造路由结构风险大于收益（M0-b 已经要改一次中间件了，别叠加）。
5. 审计日志：`log.Printf` 结构化输出 `event=backup.download user=%d file=%s ip=%s`。M1 阶段不建表。

**验收**

```bash
# 正常
curl -s :8080/api/backup/list -H "$AUTH_ADMIN" | jq .
curl -s -OJ ":8080/api/backup/download?name=$(第一个备份名)" -H "$AUTH_ADMIN"
sha256sum 下载的文件    # 与 list 返回一致

# 穿越攻击
curl -s -o /dev/null -w "%{http_code}\n" ":8080/api/backup/download?name=../../etc/passwd" -H "$AUTH_ADMIN"   # 400
curl -s -o /dev/null -w "%{http_code}\n" ":8080/api/backup/download?name=..%2F..%2Fetc%2Fpasswd" -H "$AUTH_ADMIN"  # 400

# 越权
curl -s -o /dev/null -w "%{http_code}\n" :8080/api/backup/list -H "$AUTH_NORMAL"   # 403
```

---

### M1-e · 前端备份卡片

**目标**：管理员在设置页看到备份状态并操作；普通成员看不到。

**改动文件**

| 操作 | 文件 |
|---|---|
| 新增 | `frontend/lib/screens/settings/backup_card.dart` |
| 修改 | `frontend/lib/screens/settings/settings_screen.dart`（插入卡片） |
| 修改 | `frontend/lib/core/api/ledger_api.dart`（3 个方法 + `isAdmin` 状态） |
| 修改 | `frontend/lib/viewmodels/auth_controller.dart`（存 `isAdmin`） |

**实现要点**

1. `LedgerApi` 新增：
   ```dart
   static bool _isAdmin = false;
   static bool get isAdmin => _isAdmin;
   // me() 拿到 is_admin 后写入；clearToken() 时置 false
   Future<List<BackupItem>> listBackups();
   Future<void> runBackup();
   Future<List<int>> downloadBackup(String name);   // 复用 exportArchive 的下载写法
   ```
2. `me()` 解析 `is_admin` → `LedgerApi.setIsAdmin(v)`。**注意**：`me()` 在所有登录路径都会调用（登录 + init），要确保两处都写入。
3. `BackupCard`（`StatefulWidget`）：
   - `isAdmin == false` → `return const SizedBox.shrink()`（**不渲染，不是灰掉**）
   - 加载态：`CircularProgressIndicator`
   - 数据：最近备份时间（相对时间「2 小时前」）、份数、总占用（MB）
   - 失败态：红色横幅 + 错误文案（映射成中文，不直接显示异常）
   - 按钮：「立即备份」（loading 防重复点击）、「下载最新备份」
   - 展开列表：每个文件名 + 大小 + 下载图标
4. 错误映射：401→「登录已失效，请重新登录」；403→「需要管理员权限」；其他→「备份操作失败，请查看服务端日志」。
5. **遵守约定**：`AppScope.of(context)` 只能在 `build`/`didChangeDependencies` 取；`initState` 里不能取。

**验收**

- 管理员登录 → 设置页见「数据备份」卡片，时间正确
- 普通成员登录 → 设置页无该卡片（F12 也查不到 DOM 节点）
- 点「立即备份」→ 按钮禁用 + 转圈 → 完成后时间刷新
- 下载 → 浏览器落文件，大小与列表一致
- 断网状态 → 卡片显示离线提示，不崩溃

---

## 五、新增环境变量总表

| 变量 | 默认值 | 归属 | 说明 |
|---|---|---|---|
| `LEDGER_WEB_ROOT` | `./web` | M0-b | 前端产物目录；不存在则不托管前端，服务仍启动 |
| `BEAN_CHECK_STRICT` | 空 | M0-a | `1` 时强制校验，缺失即失败（CI 必设） |
| `LEDGER_BACKUP_ENABLED` | `true` | M1-c | 关闭自动备份（测试用） |
| `LEDGER_BACKUP_DIR` | `{DBPath 目录}/backups` | M1-c | 可指向外部挂载卷 |
| `LEDGER_BACKUP_HOUR` | `3` | M1-c | 每日触发小时（0-23，按容器时区） |
| `LEDGER_BACKUP_KEEP_DAILY` | `14` | M1-c | 每日备份保留份数 |
| `LEDGER_BACKUP_KEEP_MONTHLY` | `12` | M1-c | 月度首日备份保留份数 |

> `config.Load()` 需补**启动校验**（当前完全没有）：`BackupHour` 必须在 0-23、两个 KEEP 必须 > 0、`WebRoot` 若非空但目录不存在只警告不失败。校验失败 → `log.Fatalf` 快速失败。

---

## 六、接口契约变更（前后端必须同步）

| 端点 | 变更 | 前端影响 |
|---|---|---|
| `GET /api/auth/me` | 响应新增 `is_admin` | 需解析并缓存，控制备份卡片可见性 |
| `GET /api/backup/list` | 新增 | 新方法 `listBackups()` |
| `GET /api/backup/download` | 新增 | 新方法 `downloadBackup()`（二进制） |
| `POST /api/backup/run` | 新增 | 新方法 `runBackup()` |
| `GET /*`（非 `/api/`） | **行为变更**：从「401」变为「静态文件/SPA」 | 无（前端原本就独立托管） |
| `GET /api/*` 未知路径 | 建议返回 404 JSON（现状 404 文本） | 无影响，但更规范 |

---

## 七、执行顺序与检查点

```
① ~~M1-b 前置验证：VACUUM INTO 在 modernc.org/sqlite 是否可用？~~
   ✅ 已完成（见 §1.1）：字面量与参数化均可用，快照数据完整；目标已存在会报错；cp 主文件得 0 行。
   → 结论：按原方案直接实施，无需 fallback。
        ↓
② M0-a  bean-check 严格化        ← 先有护栏
        ↓
③ M0-b  静态托管 + 中间件改造     ← 最易踩坑，单独做、单独验
        ↓
④ M0-c  Dockerfile              ← 依赖 ③ 的 WebRoot 生效
        ↓
⑤ M0-d  版本号对齐 + 校验脚本
        ↓
⑥ M0-e  workflow + compose + Makefile  ← 依赖 ②④⑤
        ↓
⑦ M1-a  迁移 008 + admin
        ↓
⑧ M1-b  快照引擎（依赖 ⑦ 的权限模型；VACUUM INTO 已实测可用）
        ↓
⑨ M1-c  调度 + 保留
        ↓
⑩ M1-d  备份 API
        ↓
⑪ M1-e  前端卡片
```

**中途可交付点**：
- 做完 ② → 账芯有了真护栏（独立价值）
- 做完 ⑥ → 有可跑的镜像（独立价值）
- 做完 ⑨ → 自动备份已生效（**此时数据已经开始被保护**，前端还没入口也不影响）
- 做完 ⑪ → M1 完整

> ⑨ 是个重要的心理节点：从这一刻起即使不做前端，数据也已经每天在被备份了。

---

## 八、冒烟脚本设计（`scripts/smoke.sh`）

覆盖一条完整链路，`make smoke` 调用：

```
1. 起服务（临时 db + 临时 web root）
2. GET /api/health                    → 期望 {"status":"ok"}
3. POST /api/auth/register            → 取 token
4. GET /api/auth/me                   → 断言 is_admin == true
5. POST /api/accounts（建资产 + 支出账户）
6. POST /api/transactions（-10 / +10 平衡）
7. GET /api/transactions              → 断言 1 条
8. GET /api/export                    → 存文件 → 跑 bean-check（严格）
9. POST /api/backup/run               → 取备份名
10. GET /api/backup/download          → 下载 → 校验 SHA256 与 run 返回一致
11. GET /api/accounts 无 token        → 断言 401
12. 停服务 → 清理临时文件
```

**要求**：脚本必须**自清理**（`trap` 删除临时目录、kill 端口），失败时保留现场并打印日志位置。

---

## 九、交付前检查清单

- [ ] `go vet ./...` 零告警
- [ ] `go test ./...` 全绿，且 **`BEAN_CHECK_STRICT=1` 下仍全绿**（证明校验真的跑了）
- [ ] `flutter analyze lib` 无 error
- [ ] `make smoke` 通过
- [ ] `docker build` 成功，镜像 < 120MB，容器非 root，重启数据不丢
- [ ] 双架构镜像能在 arm64 上 `docker run` 起来（至少验证一次）
- [ ] `scripts/verify-version.sh` 在一致时 0、漂移时 1
- [ ] 备份快照能通过 `integrity_check`，且 **WAL 未 checkpoint 的数据在快照中存在**（关键测试：写入 N 条不做 checkpoint → 快照 → 断言 N 条齐全；对照组直接 cp 主文件应得 **0 条**，已实测）
- [ ] 路径穿越 8 种变体全部返回 400（`../`、`..%2F`、绝对路径、含 `\`、空 name、超长 name、非白名单后缀、目录名）
- [ ] 非 admin 访问三个备份端点全部 403
- [ ] 旧库（无 `is_admin` 列）升级后自动补选 admin，且业务不受影响
- [ ] `AGENTS.md` 的端点表、环境变量表同步更新（否则又是文档漂移）
- [ ] `CHANGELOG.md` 记录 0.5.0 变更

---

## 十、待确认（阻塞项）

| # | 问题 | 阻塞什么 | 建议 |
|---|---|---|---|
| Q-1 | **镜像名定什么？** 现用 `famben` 占位 | workflow + compose + 文档三处 | 与仓库名统一，避免以后再改 |
| Q-2 | **FamBean 代码放哪个仓库/分支？** `origin/master` 现为 MoneyMint | CI 落在哪、是否会覆盖老项目历史 | 新建独立仓库最干净 |
| Q-3 | **Docker Hub secrets 是否已在新仓库配好？** | CI 推不上去 | `DOCKER_USERNAME` / `DOCKERHUB_TOKEN` 沿用原命名 |
| Q-4 | **首发版本号定为 0.5.0 可以吗？** | 版本校验一上线就生效，需一次性对齐三处 | 建议 0.5.0（接续 0.4.0） |

> 这 4 项不影响写代码（②③⑦⑧⑨ 可以先做），只影响 ⑤⑥ 的落地位置。**建议先开工 ①②③，同时等 Q-1/Q-2 答复。**

---

## 十一、遗留与后续（不在本次范围）

| 项 | 归属 | 说明 |
|---|---|---|
| 优雅停机 | M1-c 顺带做 | `main.go` 改用 `http.Server` + `Shutdown`，为备份定时器提供停止点 |
| 备份加密 | 暂不做 | 内网自持，加密引入密钥管理复杂度；公开后再评估 |
| 备份异地同步 | 暂不做 | 用户自行把 `backups/` 挂到同步盘 |
| 审计表 | M3 | M1 阶段只写日志，不建表 |
| 限流 / 强制改密 | M3 | 本任务书不含 |
| Beancount 导入 / 恢复 | M2 | 备份已为 M2 铺好路（快照即可直接打开） |

> 合规声明：本方案为产品设计参考，具体实施请结合公司合规与法务要求确认。
