<script setup lang="ts">
/**
 * P-06 回收站（批 2）：软删交易列表 + 恢复
 * 后端 M3 端点已有：GET /api/transactions/trash、POST /api/transactions/{uuid}/restore。
 */
import { computed, onMounted, ref, watch } from 'vue'

import { transactionApi } from '@/api/client'
import { ApiError } from '@/api/errors'
import type { TrashItemView } from '@/api/types'
import { formatMoney } from '@/core/Money'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()

const items = ref<TrashItemView[]>([])
const loading = ref(false)
const loadError = ref('')
const notice = ref('')
const busyUuid = ref<string | null>(null)

const sorted = computed(() =>
  items.value.slice().sort((a, b) => (a.deleted_at < b.deleted_at ? 1 : -1)),
)

function sumPostings(item: TrashItemView): string {
  // 摘要展示：逐币种拼接「符号+金额」，金额用后端定点原值。
  const parts: string[] = []
  for (const posting of item.postings) {
    parts.push(formatMoney(posting.amount, posting.commodity, { signed: true }))
  }
  return parts.join(' / ')
}

async function load(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    const res = await transactionApi.trash()
    items.value = Array.isArray(res.transactions) ? res.transactions : []
  } catch (err) {
    loadError.value = err instanceof ApiError ? err.message : '回收站加载失败'
  } finally {
    loading.value = false
  }
}

async function restore(item: TrashItemView): Promise<void> {
  if (busyUuid.value !== null) {
    return
  }
  busyUuid.value = item.uuid
  notice.value = ''
  try {
    await transactionApi.restore(item.uuid)
    notice.value = `已恢复「${item.description}」`
    await load()
  } catch (err) {
    notice.value = err instanceof ApiError ? `恢复失败：${err.message}` : '恢复失败，请稍后重试'
  } finally {
    busyUuid.value = null
  }
}

watch(
  () => session.ledgerId,
  () => {
    items.value = []
    void load()
  },
)

onMounted(() => {
  void load()
})
</script>

<template>
  <section class="trash">
    <header class="trash__hero fb-hero">
      <span class="trash__label">回收站 · 已删除 {{ items.length }} 笔</span>
    </header>

    <div v-if="loadError" class="fb-card trash__state trash__state--error">
      <p>{{ loadError }}</p>
      <button class="fb-btn-ghost" type="button" @click="load">重新加载</button>
    </div>
    <div v-else-if="loading" class="fb-card trash__state">正在加载回收站…</div>

    <div v-else class="fb-card trash__card">
      <p v-if="notice" class="trash__notice" role="status">{{ notice }}</p>
      <p v-if="sorted.length === 0" class="trash__empty">回收站是空的。</p>
      <ul v-else class="trash__list">
        <li v-for="item in sorted" :key="item.uuid" class="trash__row">
          <span class="trash__info">
            <span class="trash__desc">{{ item.description }}</span>
            <span class="trash__meta">{{ item.date }} · 删除于 {{ item.deleted_at }} · {{ item.created_by_name }}</span>
            <span class="trash__meta trash__postings">{{ sumPostings(item) }}</span>
          </span>
          <button
            type="button"
            class="fb-btn-ghost trash__restore"
            :disabled="busyUuid !== null"
            @click="restore(item)"
          >{{ busyUuid === item.uuid ? '恢复中…' : '恢复' }}</button>
        </li>
      </ul>
    </div>
  </section>
</template>

<style scoped>
.trash {
  padding-bottom: 20px;
}

.trash__hero {
  padding: calc(20px + env(safe-area-inset-top, 0px)) 20px 20px;
  border-radius: 0 0 24px 24px;
}

.trash__label {
  font-size: 15px;
  font-weight: 700;
}

.trash__card {
  margin-top: 14px;
  padding: 14px 16px;
}

.trash__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 0;
  border-bottom: 1px solid var(--fb-line);
}

.trash__row:last-child {
  border-bottom: none;
}

.trash__info {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.trash__desc {
  font-size: 13px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.trash__meta {
  font-size: 11px;
  color: var(--fb-sub);
  font-variant-numeric: var(--fb-font-numeric);
}

.trash__postings {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.trash__restore {
  flex-shrink: 0;
}

.trash__empty {
  margin: 0;
  padding: 16px 0;
  text-align: center;
  font-size: 12px;
  color: var(--fb-sub);
}

.trash__notice {
  margin: 0 0 10px;
  font-size: 12px;
  color: var(--fb-blue);
}

.trash__state {
  margin-top: 14px;
  padding: 20px 16px;
  text-align: center;
  font-size: 13px;
  color: var(--fb-sub);
}

.trash__state--error {
  color: var(--fb-danger);
}

.trash__state p {
  margin: 0 0 12px;
}
</style>
