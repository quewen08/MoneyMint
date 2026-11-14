<script setup lang="ts">
/**
 * P-19 导入向导（M2-f，批 4）
 *
 * Beancount 文本 → 新建独立账本。
 * - 两种输入：粘贴文本 / 上传 .beancount 文件（读为文本后走同一预检）；
 * - 预检只读不写库：valid=false 时整体拒绝并列出**全部**错误行号
 *   （RK-M2-04：跳过等于悄悄丢账，绝不宽松跳过）；
 * - valid=true 展示账本名（同名自动加序号）、账户/交易/价格/币种计数，
 *   疑似重复导入（同名 + 同交易数）仅提示不阻断；
 * - 执行在服务端单一大事务内完成，失败不留半个账本；
 *   成功后提示可到「账本」页切换查看。
 */
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

import { importApi } from '@/api/client'
import { ApiError } from '@/api/errors'
import type { ImportPreviewResult, ImportResult } from '@/api/types'
import {
  formatImportErrors,
  importSummaryRows,
  mapImportError,
  type ImportSummaryRow,
} from '@/core/m2'

const router = useRouter()

const text = ref('')
const filename = ref('')
const preview = ref<ImportPreviewResult | null>(null)
const result = ref<ImportResult | null>(null)
const busy = ref(false)
const actionError = ref('')
const fileInput = ref<HTMLInputElement | null>(null)

const hasText = computed(() => text.value.trim() !== '')
const summaryRows = computed<ImportSummaryRow[]>(() =>
  preview.value === null ? [] : importSummaryRows(preview.value),
)
const errorLines = computed<string[]>(() =>
  preview.value !== null && preview.value.errors !== undefined
    ? formatImportErrors(preview.value.errors)
    : [],
)

function onFileChange(event: Event): void {
  const input = event.target as HTMLInputElement
  const file = input.files !== null && input.files.length > 0 ? input.files[0] : null
  if (file === null) {
    return
  }
  filename.value = file.name
  // Blob.text() Safari 14 起可用，不在 §14.7 禁用清单内。
  void file
    .text()
    .then((content) => {
      text.value = content
      actionError.value = ''
    })
    .catch(() => {
      actionError.value = '读取文件失败，请重试或直接粘贴文本'
    })
}

function clearInput(): void {
  text.value = ''
  filename.value = ''
  preview.value = null
  result.value = null
  actionError.value = ''
  if (fileInput.value !== null) {
    fileInput.value.value = ''
  }
}

async function runPreview(): Promise<void> {
  if (!hasText.value || busy.value) {
    return
  }
  busy.value = true
  actionError.value = ''
  result.value = null
  try {
    preview.value = await importApi.preview(text.value)
  } catch (err) {
    actionError.value =
      err instanceof ApiError ? mapImportError(err.status, err.message) : mapImportError(0, '')
  } finally {
    busy.value = false
  }
}

async function runImport(): Promise<void> {
  if (!hasText.value || busy.value) {
    return
  }
  busy.value = true
  actionError.value = ''
  try {
    result.value = await importApi.execute(text.value)
    preview.value = null
  } catch (err) {
    actionError.value =
      err instanceof ApiError ? mapImportError(err.status, err.message) : mapImportError(0, '')
  } finally {
    busy.value = false
  }
}

function gotoLedgers(): void {
  void router.push({ name: 'ledgers' })
}
</script>

<template>
  <section class="im">
    <header class="im__hero fb-hero">
      <span class="im__title">导入 Beancount</span>
      <span class="im__sub">解析为独立新账本，不覆盖既有数据</span>
    </header>

    <p v-if="actionError" class="im__notice" role="alert">{{ actionError }}</p>

    <!-- 输入 -->
    <div class="fb-card im__card">
      <h2>① 提供文件内容</h2>
      <p class="im__desc">支持粘贴文本或上传 .beancount 文件；仅支持本系统导出用到的语法子集。</p>
      <input ref="fileInput" class="im__file" type="file" accept=".beancount,.txt,text/plain" @change="onFileChange" />
      <textarea
        v-model="text"
        class="im__textarea"
        rows="8"
        spellcheck="false"
        placeholder='option "title" "家庭账本"&#10;2026-11-01 open Assets:Cash:CNY&#10;2026-11-02 * "早餐"&#10;  Expenses:食品:CNY -12.50 CNY&#10;  Assets:Cash:CNY'
      ></textarea>
      <p v-if="filename !== ''" class="im__desc">已读入文件：{{ filename }}</p>
      <div class="im__buttons">
        <button class="fb-btn-primary im__btn" type="button" :disabled="!hasText || busy" @click="runPreview">
          {{ busy ? '解析中…' : '解析预览' }}
        </button>
        <button class="fb-btn-ghost im__btn" type="button" :disabled="busy" @click="clearInput">清空</button>
      </div>
    </div>

    <!-- 预检结果 -->
    <template v-if="preview !== null">
      <div v-if="!preview.valid" class="fb-card im__card im__reject" role="alert">
        <h2>解析失败，已整体拒绝</h2>
        <p class="im__desc">{{ preview.error || '文件含不支持的语法，已整体拒绝' }}</p>
        <ul class="im__errors">
          <li v-for="line in errorLines" :key="line">{{ line }}</li>
        </ul>
        <p class="im__desc">修改文件后重新解析即可；不会产生任何账本数据。</p>
      </div>

      <div v-else class="fb-card im__card">
        <h2>② 确认导入内容</h2>
        <table class="im__table">
          <tbody>
            <tr v-for="row in summaryRows" :key="row.label">
              <td>{{ row.label }}</td>
              <td>{{ row.value }}</td>
            </tr>
          </tbody>
        </table>
        <p class="im__desc">将创建新账本「{{ preview.ledger_name }}」<span v-if="preview.title !== ''">（来源 title：{{ preview.title }}）</span>。</p>
        <p v-if="preview.duplicate" class="im__must" role="alert">
          已存在同名账本且交易数相同，疑似重复导入；确认无误可继续（不会覆盖原账本）。
        </p>
        <div class="im__buttons">
          <button class="fb-btn-primary im__btn" type="button" :disabled="busy" @click="runImport">
            {{ busy ? '导入中…' : '执行导入' }}
          </button>
          <button class="fb-btn-ghost im__btn" type="button" :disabled="busy" @click="clearInput">取消</button>
        </div>
      </div>
    </template>

    <!-- 导入结果 -->
    <div v-if="result !== null" class="fb-card im__card">
      <h2>导入完成</h2>
      <p class="im__desc">
        新账本「{{ result.ledger_name }}」：账户 {{ result.accounts }} · 交易 {{ result.transactions }}。
      </p>
      <div class="im__buttons">
        <button class="fb-btn-primary im__btn" type="button" @click="gotoLedgers">去账本页查看</button>
        <button class="fb-btn-ghost im__btn" type="button" @click="clearInput">再导入一份</button>
      </div>
    </div>
  </section>
</template>

<style scoped>
.im { padding-bottom: 20px; }
.im__hero { padding: calc(20px + env(safe-area-inset-top, 0px)) 20px 20px; border-radius: 0 0 24px 24px; }
.im__title { display: block; font-size: 16px; font-weight: 700; }
.im__sub { display: block; margin-top: 4px; font-size: 12px; color: var(--fb-on-hero-muted); }
.im__card { margin-top: 12px; padding: 14px 16px; }
.im__card h2 { margin: 0 0 8px; font-size: 14px; }
.im__reject { border: 1px solid var(--fb-danger); }
.im__reject h2 { color: var(--fb-danger); }
.im__desc { margin: 0 0 8px; font-size: 11px; color: var(--fb-sub); line-height: 1.7; }
.im__notice { margin: 10px 0 0; padding: 8px 12px; border-radius: var(--fb-radius-input); font-size: 12px; background: var(--fb-danger-soft); color: var(--fb-danger); }
.im__file { display: block; margin: 10px 0; font-size: 12px; }
.im__textarea { width: 100%; box-sizing: border-box; padding: 10px; border: 1.5px solid var(--fb-line); border-radius: var(--fb-radius-input); font-size: 12px; font-family: monospace; line-height: 1.6; resize: vertical; background: var(--fb-surface, transparent); color: var(--fb-text, inherit); }
.im__buttons { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 10px; }
.im__btn { padding: 6px 12px; font-size: 12px; }
.im__table { width: 100%; border-collapse: collapse; font-size: 12px; margin-bottom: 10px; }
.im__table td { padding: 6px 8px; border-bottom: 1px solid var(--fb-line); }
.im__table td:first-child { color: var(--fb-sub); width: 40%; }
.im__errors { margin: 0 0 10px; padding-left: 18px; font-size: 11px; line-height: 1.8; color: var(--fb-danger); }
.im__must { margin: 10px 0 0; padding: 8px 12px; border-radius: var(--fb-radius-input); font-size: 12px; background: var(--fb-warn-soft); color: var(--fb-warn); font-weight: 600; }
</style>
