<script setup lang="ts">
/**
 * 密码输入框（带「显示 / 隐藏密码」切换，AC-36 配套）
 *
 * 设计要点：
 * - 默认 `type="password"`，点击右侧眼睛按钮在 `password` / `text` 间切换；
 *   切换后主动把焦点与光标放回输入框末尾，避免移动端键盘收起。
 * - 眼睛图标用内联 SVG 自绘，**不引入图标库**；按钮 `type="button"` 防止
 *   在 form 内触发提交，`aria-label` / `aria-pressed` 随状态同步。
 * - 输入框保留全局 `fb-input` 类，但字号强制 16px：iOS Safari 对 <16px 的
 *   输入框会在聚焦时自动放大页面。
 * - 兼容基线 Safari 14.1：不使用 aspect-ratio / :has() / dvh / inset。
 */
import { computed, nextTick, ref } from 'vue'

const props = withDefaults(
  defineProps<{
    /** 密码值（v-model） */
    modelValue: string
    /** 浏览器自动填充提示，如 current-password / new-password */
    autocomplete?: string
    /** 占位文案 */
    placeholder?: string
    /** 禁用态（输入框与切换按钮同时禁用） */
    disabled?: boolean
    /** 输入框 id，便于 <label for> 关联 */
    id?: string
  }>(),
  {
    autocomplete: undefined,
    placeholder: '',
    disabled: false,
    id: undefined,
  },
)

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()

/** 是否明文显示（false = password，true = text） */
const visible = ref(false)
const inputRef = ref<HTMLInputElement | null>(null)

const inputType = computed<string>(() => (visible.value ? 'text' : 'password'))
const toggleLabel = computed<string>(() => (visible.value ? '隐藏密码' : '显示密码'))

/** 与父级 v-model 双向绑定 */
const model = computed<string>({
  get: () => props.modelValue,
  set: (value: string) => {
    emit('update:modelValue', value)
  },
})

/**
 * 切换明/密文：先等 DOM 重渲染（type 变化会让部分浏览器重置选区），
 * 再聚焦并把光标移到末尾。
 */
async function toggle(): Promise<void> {
  visible.value = !visible.value
  await nextTick()
  const el = inputRef.value
  if (el === null) {
    return
  }
  el.focus()
  try {
    const end = el.value.length
    el.setSelectionRange(end, end)
  } catch {
    // 少数浏览器对某些 input 类型不支持 setSelectionRange，忽略即可。
  }
}
</script>

<template>
  <span class="pwd-input">
    <input
      ref="inputRef"
      :id="id"
      v-model="model"
      class="fb-input pwd-input__field"
      :type="inputType"
      :autocomplete="autocomplete"
      :placeholder="placeholder"
      :disabled="disabled"
      autocapitalize="off"
      autocorrect="off"
      spellcheck="false"
    />
    <button
      class="pwd-input__toggle"
      type="button"
      :aria-label="toggleLabel"
      :aria-pressed="visible ? 'true' : 'false'"
      :title="toggleLabel"
      :disabled="disabled"
      @click="toggle"
    >
      <!-- 隐藏态：睁眼图标，点击后显示密码 -->
      <svg
        v-if="!visible"
        class="pwd-input__icon"
        viewBox="0 0 24 24"
        width="20"
        height="20"
        fill="none"
        stroke="currentColor"
        stroke-width="1.8"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
        focusable="false"
      >
        <path d="M1.8 12S5.6 5.4 12 5.4 22.2 12 22.2 12 18.4 18.6 12 18.6 1.8 12 1.8 12Z" />
        <circle cx="12" cy="12" r="3" />
      </svg>
      <!-- 明文态：划线眼睛图标，点击后隐藏密码 -->
      <svg
        v-else
        class="pwd-input__icon"
        viewBox="0 0 24 24"
        width="20"
        height="20"
        fill="none"
        stroke="currentColor"
        stroke-width="1.8"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
        focusable="false"
      >
        <path d="M1.8 12S5.6 5.4 12 5.4 22.2 12 22.2 12 18.4 18.6 12 18.6 1.8 12 1.8 12Z" />
        <circle cx="12" cy="12" r="3" />
        <path d="M3.6 3.6 20.4 20.4" />
      </svg>
    </button>
  </span>
</template>

<style scoped>
.pwd-input {
  position: relative;
  display: block;
  width: 100%;
}

/* 覆盖 fb-input 的 15px 与右内边距：16px 防 iOS 聚焦自动放大，右侧留给按钮 */
.pwd-input .pwd-input__field {
  width: 100%;
  padding-right: 48px;
  font-size: 16px;
}

.pwd-input__toggle {
  position: absolute;
  top: 50%;
  right: 3px;
  transform: translateY(-50%);
  width: 40px;
  height: 40px;
  padding: 0;
  border: none;
  background: transparent;
  color: var(--fb-sub);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  border-radius: var(--fb-radius-input);
  -webkit-tap-highlight-color: transparent;
}

.pwd-input__toggle:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.pwd-input__icon {
  display: block;
  pointer-events: none;
}
</style>
