#!/bin/sh
# FamBean 容器入口：自动修正数据目录属主，然后降权运行服务。
#
# 为什么需要它：
#   镜像以非 root 用户（uid 10001）运行服务，但宿主机 bind mount 过来的目录
#   通常由 docker/root 创建，属主不是 10001。SQLite 在 WAL 模式下要在库文件
#   同目录创建 -wal/-shm，目录不可写就会报 "unable to open database file (14)"。
#
#   让使用者每次部署都手工 chown 一遍不现实（NAS 上还常被 ACL 覆盖），
#   所以在启动瞬间以 root 修一次属主，然后立刻用 su-exec 降权到 10001 执行。
#   这与多数官方镜像（nginx、postgres 等）的做法一致：只有入口脚本是 root，
#   真正的服务进程始终是非 root。
#
# 若使用者通过 compose 的 `user:` 指定了非 root 用户启动，则跳过修正直接执行，
# 保留「全程非 root」的部署方式。

set -e

APP_UID=10001
APP_GID=10001

# fix_owner 把目录属主改为运行用户；已正确则跳过，避免每次启动都递归 chown。
fix_owner() {
    dir="$1"
    [ -n "$dir" ] || return 0
    mkdir -p "$dir" 2>/dev/null || true
    [ -d "$dir" ] || return 0

    current=$(stat -c '%u' "$dir" 2>/dev/null || echo "")
    if [ "$current" != "$APP_UID" ]; then
        echo "[entrypoint] 修正数据目录属主: $dir -> $APP_UID:$APP_GID"
        chown -R "$APP_UID:$APP_GID" "$dir" 2>/dev/null \
            || echo "[entrypoint] ⚠️ 修正属主失败（可能是只读挂载或 ACL 限制）: $dir"
    fi
}

if [ "$(id -u)" = "0" ]; then
    # 数据库所在目录 + 备份目录（若单独指定）
    fix_owner "$(dirname "${LEDGER_DB:-/app/data/ledger.db}")"
    if [ -n "${LEDGER_BACKUP_DIR:-}" ]; then
        fix_owner "$LEDGER_BACKUP_DIR"
    fi

    # 降权执行；exec 保证 PID 1 就是服务进程，信号能正确传递
    exec su-exec "$APP_UID:$APP_GID" "$@"
fi

# 非 root 启动（compose 指定了 user:）：不做修正，直接执行
exec "$@"
