<script setup lang="ts">
/**
 * P-20 恢复向导（M2-c，批 4）
 *
 * 三步流：选文件 → 只读预检 → 确认词执行：
 * - 预检不改动任何数据，摘要卡展示「备份 vs 当前」覆盖量对比 + 副作用警告；
 * - valid=false 时整体拒绝并给出原因（损坏 / 非本系统备份 / 未来版本 / 超大）；
 * - 「执行还原」必须输入 RESTORE 解锁（服务端同样硬校验，双保险）；
 * - 成功后展示替换行数、pre-restore 快照名与重写后的库世代，
 *   并明确提示「请用备份中的账号重新登录」——本机登录态随用户表被替换而失效。
 *
 * 全库级操作：路由带 adminOnly 守卫，视图内再断言一次 is_admin（纵深防御）。
 */
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

import { restoreApi } from '@/api/client'
import { ApiError } from '@/api/errors'
import type { RestorePreviewResult, RestoreResult } from '@/api/types'
import {
  RESTORE_CONFIRM_WORD,
  mapRestoreError,
  restoreCompareRows,
  restoreRejectionText,
} from '@/core/m2'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const router = useRouter()

type Step = 'pick' | 'preview' | 'done'

const step = ref<Step>('pick')
const file = ref<File | null>(null)
const preview = ref<RestorePreviewResult | null>(null)
const result = ref<RestoreResult | null>(null)
const confirmText = ref('')
const busy = ref(false)
const actionError = ref('')
const fileInput = ref<HTMLInputElement | null>(null)

const isAdmin = computed(() => session.user !== null && session.user.is_admin === true)
const confirmUnlocked = computed(() => confirmText.value === RESTORE_CONFIRM_WORD)
const compareRows = computed(() =>
  preview.value === null ? [] : restoreCompareRows(preview.value.backup, preview.value.current),
)

function onFileChange(event: Event): void {
  const input = event.target as HTMLInputElement
  file.value = input.files !== null && input.files.length > 0 ? input.files[0] : null
  actionError.value = ''
}

function reset(): void {
  step.value = 'pick'
  file.value = null
  preview.value = null
  result.value = null
  confirmText.value = ''
  actionError.value = ''
  if (fileInput.value !== null) {
    fileInput.value.value = ''
  }
}

async function runPreview(): Promise<void> {
  if (file.value === null || busy.value) {
    return
  }
  busy.value = true
  actionError.value = ''
  try {
    preview.value = await restoreApi.preview(file.value)
    step.value = 'preview'
  } catch (err) {
    actionError.value =
      err instanceof ApiError ? mapRestoreError(err.status, err.message) : mapRestoreError(0, '')
  } finally {
    busy.value = false
  }
}

async function runRestore(): Promise<void> {
  if (file.value === null || !confirmUnlocked.value || busy.value) {
    return
  }
  busy.value = true
  actionError.value = ''
  try {
    result.value = await restoreApi.execute(file.value, confirmText.value)
    step.value = 'done'
  } catch (err) {
    actionError.value =
      err instanceof ApiError ? mapRestoreError(err.status, err.message) : mapRestoreError(0, '')
  } finally {
    busy.value = false
  }
}

/** 还原成功后本机登录态已随用户表被替换：仅本地登出，不再调 logout 端点。 */
function backToLogin(): void {
  session.markGuest()
  void router.replace({ name: 'login' })
}
</script>

<template>
  <section class="rs">
    <header class="rs__hero fb-hero">
      <span class="rs__title">还原备份</span>
      <span class="rs__sub">从 .db 备份还原全库</span>
    </header>

    <div v-if="!isAdmin" class="fb-card rs__card" role="alert">
      <h2>仅系统管理员可还原备份</h2>
      <p>还原会替换全部账本与用户数据。请联系系统管理员操作。</p>
    </div>

    <template v-else>
      <p v-if="actionError" class="rs__notice" role="alert">{{ actionError }}</p>

      <!-- 第 1 步：选择备份文件 -->
      <div v-if="step === 'pick'" class="fb-card rs__card">
        <h2>① 选择备份文件</h2>
        <p class="rs__desc">支持本系统生成的 .db 备份文件（含从其他设备下载的副本）。</p>
        <input
          ref="fileInput"
          class="rs__file"
          type="file"
          accept=".db,application/octet-stream"
          @change="onFileChange"
        />
        <div class="rs__buttons">
          <button class="fb-btn-primary rs__btn" type="button" :disabled="file === null || busy" @click="runPreview">
            {{ busy ? '预检中…' : '开始预检' }}
          </button>
        </div>
      </div>

      <!-- 第 2 步：预检摘要 + 确认词解锁 -->
      <template v-else-if="step === 'preview' && preview !== null">
        <div v-if="!preview.valid" class="fb-card rs__card rs__reject" role="alert">
          <h2>预检未通过，已拒绝还原</h2>
          <p>{{ restoreRejectionText(preview) }}</p>
          <div class="rs__buttons">
            <button class="fb-btn-ghost rs__btn" type="button" @click="reset">重新选择文件</button>
          </div>
        </div>

        <div v-else class="fb-card rs__card">
          <h2>② 确认覆盖范围</h2>
          <table class="rs__table">
            <thead>
              <tr><th>内容</th><th>备份中</th><th>当前</th></tr>
            </thead>
            <tbody>
              <tr v-for="row in compareRows" :key="row.label">
                <td>{{ row.label }}</td>
                <td>{{ row.backup }}</td>
                <td>{{ row.current }}</td>
              </tr>
            </tbody>
          </table>
          <p class="rs__desc">schema 版本：备份 v{{ preview.schema_version }} / 当前 v{{ preview.current_version }}</p>
          <ul class="rs__warnings">
            <li v-for="w in preview.warnings" :key="w">{{ w }}</li>
          </ul>
          <label class="rs__confirm">
            <span>输入 <strong>{{ RESTORE_CONFIRM_WORD }}</strong> 解锁执行：</span>
            <input v-model="confirmText" class="rs__input" type="text" autocomplete="off" :placeholder="RESTORE_CONFIRM_WORD" />
          </label>
          <div class="rs__buttons">
            <button class="fb-btn-primary rs__btn" type="button" :disabled="!confirmUnlocked || busy" @click="runRestore">
              {{ busy ? '还原中…' : '执行还原' }}
            </button>
            <button class="fb-btn-ghost rs__btn" type="button" :disabled="busy" @click="reset">取消</button>
          </div>
        </div>
      </template>

      <!-- 第 3 步：结果 -->
      <div v-else-if="step === 'done' && result !== null" class="fb-card rs__card">
        <h2>还原完成</h2>
        <p class="rs__desc">
          已替换：用户 {{ result.replaced.users }} · 账本 {{ result.replaced.ledgers }} ·
          账户 {{ result.replaced.accounts }} · 交易 {{ result.replaced.transactions }}
        </p>
        <p v-if="result.snapshot" class="rs__desc">还原前快照：{{ result.snapshot }}（如需撤销，可再次还原此备份）</p>
        <p class="rs__desc">库世代已更新：{{ result.new_instance_id }}，其他设备将自动全量重新同步。</p>
        <p class="rs__must" role="alert">{{ result.warnings[0] || '请用备份中的账号重新登录' }}</p>
        <div class="rs__buttons">
          <button class="fb-btn-primary rs__btn" type="button" @click="backToLogin">重新登录</button>
        </div>
      </div>
    </template>
  </section>
</template>

<style scoped>
.rs { padding-bottom: 20px; }
.rs__hero { padding: calc(20px + env(safe-area-inset-top, 0px)) 20px 20px; border-radius: 0 0 24px 24px; }
.rs__title { display: block; font-size: 16px; font-weight: 700; }
.rs__sub { display: block; margin-top: 4px; font-size: 12px; color: var(--fb-on-hero-muted); }
.rs__card { margin-top: 12px; padding: 14px 16px; }
.rs__card h2 { margin: 0 0 8px; font-size: 14px; }
.rs__reject { border: 1px solid var(--fb-danger); }
.rs__reject h2 { color: var(--fb-danger); }
.rs__desc { margin: 0 0 8px; font-size: 11px; color: var(--fb-sub); line-height: 1.7; }
.rs__notice { margin: 10px 0 0; padding: 8px 12px; border-radius: var(--fb-radius-input); font-size: 12px; background: var(--fb-danger-soft); color: var(--fb-danger); }
.rs__file { display: block; margin: 10px 0; font-size: 12px; }
.rs__buttons { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 10px; }
.rs__btn { padding: 6px 12px; font-size: 12px; }
.rs__table { width: 100%; border-collapse: collapse; font-size: 12px; margin-bottom: 10px; }
.rs__table th, .rs__table td { padding: 6px 8px; border-bottom: 1px solid var(--fb-line); text-align: left; }
.rs__table th { color: var(--fb-sub); font-weight: 600; font-size: 11px; }
.rs__warnings { margin: 0 0 10px; padding-left: 18px; font-size: 11px; color: var(--fb-sub); line-height: 1.8; }
.rs__confirm { display: flex; align-items: center; gap: 8px; font-size: 12px; margin-top: 6px; flex-wrap: wrap; }
.rs__input { padding: 6px 8px; border: 1px solid var(--fb-line); border-radius: var(--fb-radius-input); font-size: 12px; width: 140px; }
.rs__must { margin: 10px 0 0; padding: 8px 12px; border-radius: var(--fb-radius-input); font-size: 12px; background: var(--fb-warn-soft, #fff7e6); color: var(--fb-warn, #b8860b); font-weight: 600; }
</style>
