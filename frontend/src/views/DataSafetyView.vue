<script setup lang="ts">
/**
 * P-15 数据安全（批 3）
 *
 * 与旧版 backup_card.dart + import_export_screen.dart 1:1：
 * - 备份（仅系统管理员渲染，避免暴露无权功能）：列表 / 立即备份 / 下载 /
 *   409 冲突 / 503 未启用等错误文案 / 相对时间 / 体积 / 失败记录；
 * - 导出：单文件 beancount、目录 zip、CSV（纯前端生成，字段与旧版一致）；
 * - 导入：本期占位提示（M2 的 P-19/P-20 属批 4）。
 *
 * §14.5 #3 不降级且按方案**补强**：旧版是导出后用响应头告警、不阻断；
 * 新版导出前先跑 `GET /api/identity/check`，不平即**阻断**并列出按币种差额。
 * 导出成功后仍读 `X-Ledger-*` 响应头做二次提示（双保险）。
 */
import { computed, onMounted, ref } from 'vue'

import { accountApi, backupApi, exportApi, identityApi, transactionApi } from '@/api/client'
import { ApiError } from '@/api/errors'
import type { BackupItem } from '@/api/types'
import {
  backupSucceeded,
  downloadBlob,
  downloadText,
  formatBackupSize,
  formatIdentityDiffs,
  fromIdentityDiffs,
  mapBackupError,
  relativeTime,
} from '@/core/batch3'
import { buildCsvRowsFromTransactions, buildTransactionsCsv } from '@/core/csv'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()

const items = ref<BackupItem[]>([])
const backupLoading = ref(false)
const backupBusy = ref(false)
const backupError = ref('')
const showAll = ref(false)
const notice = ref('')
const actionError = ref('')
const checking = ref(false)
const unbalanced = ref(false)
const diffText = ref('')

const isAdmin = computed(() => session.user !== null && session.user.is_admin === true)
const visibleItems = computed(() => (showAll.value ? items.value : items.value.slice(0, 3)))
const successfulItems = computed(() => items.value.filter(backupSucceeded))
const totalSize = computed(() => {
  let total = 0
  for (const item of items.value) {
    total += item.size
  }
  return total
})

function messageOf(err: unknown): string {
  return err instanceof ApiError ? err.message : '操作失败，请稍后重试'
}

async function loadBackups(): Promise<void> {
  if (!isAdmin.value) {
    return
  }
  backupLoading.value = true
  backupError.value = ''
  try {
    const res = await backupApi.list()
    items.value = Array.isArray(res.items) ? res.items : []
  } catch (err) {
    backupError.value =
      err instanceof ApiError
        ? mapBackupError(err.status, err.message)
        : mapBackupError(0, '')
  } finally {
    backupLoading.value = false
  }
}

async function runBackup(): Promise<void> {
  if (backupBusy.value) {
    return
  }
  backupBusy.value = true
  backupError.value = ''
  notice.value = ''
  try {
    await backupApi.run()
    notice.value = '备份已创建'
    await loadBackups()
  } catch (err) {
    backupError.value =
      err instanceof ApiError
        ? mapBackupError(err.status, err.message)
        : mapBackupError(0, '')
  } finally {
    backupBusy.value = false
  }
}

async function downloadBackup(name: string): Promise<void> {
  backupError.value = ''
  try {
    const blob = await backupApi.download(name)
    downloadBlob(name, blob)
    notice.value = `已下载 ${name}`
  } catch (err) {
    backupError.value =
      err instanceof ApiError
        ? mapBackupError(err.status, err.message)
        : mapBackupError(0, '')
  }
}

/** 导出前恒等式校验：不平即阻断（方案 §14.5 #3 比旧版更严）。 */
async function ensureBalanced(): Promise<boolean> {
  checking.value = true
  actionError.value = ''
  try {
    const res = await identityApi.check()
    if (!res.balanced) {
      unbalanced.value = true
      diffText.value = formatIdentityDiffs(fromIdentityDiffs(res.diffs))
      return false
    }
    unbalanced.value = false
    diffText.value = ''
    return true
  } catch (err) {
    actionError.value = `导出前恒等式检查失败：${messageOf(err)}`
    return false
  } finally {
    checking.value = false
  }
}

async function exportFile(kind: 'beancount' | 'archive'): Promise<void> {
  if (checking.value) {
    return
  }
  if (!(await ensureBalanced())) {
    return
  }
  try {
    const result = kind === 'beancount' ? await exportApi.beancount() : await exportApi.archive()
    const filename = kind === 'beancount' ? 'family.beancount' : 'ledger-beancount.zip'
    downloadBlob(filename, result.blob)
    if (!result.balanced) {
      actionError.value = `已导出，但响应头提示账本不平：${formatIdentityDiffs(result.diffs)}`
    } else {
      notice.value = `已导出 ${filename}`
    }
  } catch (err) {
    actionError.value = `导出失败（需联网）：${messageOf(err)}`
  }
}

async function exportCsv(): Promise<void> {
  try {
    const [txnRes, accountRes] = await Promise.all([transactionApi.list(), accountApi.list()])
    const rows = buildCsvRowsFromTransactions(
      Array.isArray(txnRes.transactions) ? txnRes.transactions : [],
      Array.isArray(accountRes.accounts) ? accountRes.accounts : [],
    )
    downloadText('family-ledger.csv', buildTransactionsCsv(rows), 'text/csv')
    notice.value = `已导出 CSV（${rows.length} 笔交易）`
    actionError.value = ''
  } catch (err) {
    actionError.value = `CSV 导出失败：${messageOf(err)}`
  }
}

onMounted(() => { void loadBackups() })
</script>

<template>
  <section class="ds">
    <header class="ds__hero fb-hero">
      <span class="ds__title">数据安全</span>
      <span class="ds__sub">备份 · 导出 · 导入</span>
    </header>

    <p v-if="notice" class="ds__notice ds__notice--ok" role="status">{{ notice }}</p>
    <p v-if="actionError" class="ds__notice ds__notice--err" role="alert">{{ actionError }}</p>

    <!-- 导出不平阻断：§14.5 #3 -->
    <div v-if="unbalanced" class="fb-card ds__block" role="alert">
      <h2>账本不平，已阻断导出</h2>
      <p>差额：{{ diffText || '（服务端未返回明细）' }}</p>
      <p class="fb-muted">请先修正分录，确保按币种借贷平衡且导出能通过 bean-check。</p>
    </div>

    <div class="fb-card ds__card">
      <h2>导出</h2>
      <p class="ds__desc">导出前会做恒等式检查；不平即阻断，并列出按币种差额。</p>
      <div class="ds__buttons">
        <button class="fb-btn-primary ds__btn" type="button" :disabled="checking" @click="exportFile('beancount')">
          {{ checking ? '检查中…' : '导出 Beancount' }}
        </button>
        <button class="fb-btn-ghost ds__btn" type="button" :disabled="checking" @click="exportFile('archive')">导出目录 zip</button>
        <button class="fb-btn-ghost ds__btn" type="button" @click="exportCsv">导出 CSV</button>
      </div>
    </div>

    <div class="fb-card ds__card">
      <h2>导入</h2>
      <p class="ds__desc">
        Beancount 导入与备份还原向导属于 M2（批 4），本期尚未开放。导出文件可被 bean-check 直接校验。
      </p>
    </div>

    <!-- 备份：仅系统管理员渲染 -->
    <div v-if="isAdmin" class="fb-card ds__card">
      <div class="ds__card-head">
        <h2>数据备份</h2>
        <span v-if="backupBusy" class="fb-muted">正在备份…</span>
      </div>
      <p v-if="backupLoading" class="ds__state">正在读取备份状态…</p>
      <template v-else>
        <p v-if="backupError" class="ds__err">{{ backupError }}
          <button class="fb-btn-ghost ds__btn" type="button" @click="loadBackups">重试</button>
        </p>
        <p v-else-if="items.length === 0" class="ds__state">
          尚未创建备份。建议立即备份一次，并在其他设备保存一份。
        </p>
        <p v-else class="ds__desc">
          最近备份：{{ relativeTime(items[0].created_at) }} · 成功 {{ successfulItems.length }} 份 ·
          占用 {{ formatBackupSize(totalSize) }}
        </p>

        <div class="ds__buttons">
          <button class="fb-btn-primary ds__btn" type="button" :disabled="backupBusy" @click="runBackup">立即备份</button>
          <button
            v-if="successfulItems.length > 0"
            class="fb-btn-ghost ds__btn"
            type="button"
            @click="downloadBackup(successfulItems[0].name)"
          >下载最新备份</button>
          <button
            v-if="items.length > 3"
            class="fb-btn-ghost ds__btn"
            type="button"
            @click="showAll = !showAll"
          >{{ showAll ? '收起列表' : `查看全部（${items.length}）` }}</button>
        </div>

        <ul v-if="showAll" class="ds__backups">
          <li v-for="item in visibleItems" :key="item.name">
            <span class="ds__status" :class="backupSucceeded(item) ? 'ds__status--ok' : 'ds__status--err'">
              {{ backupSucceeded(item) ? '✓' : '!' }}
            </span>
            <span class="ds__backup-name">
              {{ item.name }}
              <small>
                {{ backupSucceeded(item)
                  ? `${item.trigger === 'manual' ? '手动' : '自动'} · ${formatBackupSize(item.size)} · ${relativeTime(item.created_at)}`
                  : `失败：${item.error || '未知原因'}` }}
              </small>
            </span>
            <button
              v-if="backupSucceeded(item)"
              class="fb-btn-ghost ds__btn"
              type="button"
              @click="downloadBackup(item.name)"
            >下载</button>
          </li>
        </ul>
      </template>
      <p class="ds__hint">备份包含全部账本的完整数据，建议把文件另存到其他磁盘。</p>
    </div>
  </section>
</template>

<style scoped>
.ds { padding-bottom: 20px; }
.ds__hero { padding: calc(20px + env(safe-area-inset-top, 0px)) 20px 20px; border-radius: 0 0 24px 24px; }
.ds__title { display: block; font-size: 16px; font-weight: 700; }
.ds__sub { display: block; margin-top: 4px; font-size: 12px; color: var(--fb-on-hero-muted); }
.ds__notice { margin: 10px 0 0; padding: 8px 12px; border-radius: var(--fb-radius-input); font-size: 12px; }
.ds__notice--ok { background: var(--fb-green-soft); color: var(--fb-green); }
.ds__notice--err { background: var(--fb-danger-soft); color: var(--fb-danger); }
.ds__block { margin-top: 12px; padding: 14px 16px; border: 1px solid var(--fb-danger); }
.ds__block h2 { margin: 0 0 8px; font-size: 14px; color: var(--fb-danger); }
.ds__block p { margin: 0 0 6px; font-size: 12px; }
.ds__card { margin-top: 12px; padding: 14px 16px; }
.ds__card h2 { margin: 0 0 8px; font-size: 14px; }
.ds__card-head { display: flex; align-items: center; justify-content: space-between; }
.ds__desc, .ds__hint { margin: 0 0 10px; font-size: 11px; color: var(--fb-sub); line-height: 1.7; }
.ds__buttons { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 10px; }
.ds__btn { padding: 6px 12px; font-size: 12px; }
.ds__state { margin: 10px 0; font-size: 12px; color: var(--fb-sub); }
.ds__err { margin: 10px 0; font-size: 12px; color: var(--fb-danger); }
.ds__backups { list-style: none; margin: 12px 0 0; padding: 0; }
.ds__backups li { display: flex; align-items: center; gap: 10px; padding: 8px 0; border-bottom: 1px solid var(--fb-line); }
.ds__backups li:last-child { border-bottom: none; }
.ds__status { width: 20px; font-weight: 700; }
.ds__status--ok { color: var(--fb-green); }
.ds__status--err { color: var(--fb-danger); }
.ds__backup-name { flex: 1; min-width: 0; font-size: 12px; overflow-wrap: anywhere; }
.ds__backup-name small { display: block; color: var(--fb-sub); font-size: 11px; }
</style>
