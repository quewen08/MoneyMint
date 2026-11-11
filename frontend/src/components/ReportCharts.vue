<script setup lang="ts">
/**
 * P-11 报表图表（Canvas 2D 自绘，批 3）
 *
 * 硬约束：只用 2D context，禁 WebGL / WASM / CanvasKit / OffscreenCanvas，
 * 不引 ECharts / fl_chart 之类依赖（§6.4 / §14.7）。
 * 金额字符串只在绘图时转 Number 求坐标比例，**不参与任何金额运算或存储**；
 * 真正的金额聚合在 `core/reports.ts` 里用 decimal.js 定点完成。
 *
 * jsdom（Vitest）没有 canvas 上下文，getContext 返回 null 时静默跳过绘制。
 */
import { onMounted, ref, watch } from 'vue'

interface Share {
  label: string
  amount: string
  ratio: number
}

const props = withDefaults(
  defineProps<{
    kind: 'trend' | 'share'
    labels?: string[]
    income?: string[]
    expense?: string[]
    shares?: Share[]
    height?: number
  }>(),
  {
    labels: () => [],
    income: () => [],
    expense: () => [],
    shares: () => [],
    height: 220,
  },
)

const canvasRef = ref<HTMLCanvasElement | null>(null)

const GRID = '#e5e7eb'
const INK = '#6b7280'
const EXPENSE = '#3b6ef6'
const INCOME = '#2ba471'
const PIE_COLORS = ['#3b6ef6', '#2ba471', '#e8590c', '#8b5cf6', '#d93025', '#0ea5e9']

/** 坐标用紧凑数字（1.2k / 3.4w），避免画布被长金额挤爆。 */
function compact(value: number): string {
  const abs = Math.abs(value)
  if (abs >= 10000) {
    return `${(value / 10000).toFixed(1)}w`
  }
  if (abs >= 1000) {
    return `${(value / 1000).toFixed(1)}k`
  }
  return String(Math.round(value))
}

function drawLine(
  ctx: CanvasRenderingContext2D,
  values: string[],
  max: number,
  left: number,
  top: number,
  width: number,
  height: number,
  color: string,
): void {
  if (values.length === 0) {
    return
  }
  const step = values.length > 1 ? width / (values.length - 1) : 0
  ctx.beginPath()
  ctx.strokeStyle = color
  ctx.lineWidth = 2
  for (let i = 0; i < values.length; i += 1) {
    const magnitude = Number(values[i])
    const x = left + step * i
    const y = top + height - (max > 0 ? (magnitude / max) * height : 0)
    if (i === 0) {
      ctx.moveTo(x, y)
    } else {
      ctx.lineTo(x, y)
    }
  }
  ctx.stroke()

  // 数据点（超过 14 个桶时不画，避免密集成一团）
  if (values.length <= 14) {
    ctx.fillStyle = color
    for (let i = 0; i < values.length; i += 1) {
      const magnitude = Number(values[i])
      const x = left + step * i
      const y = top + height - (max > 0 ? (magnitude / max) * height : 0)
      ctx.beginPath()
      ctx.arc(x, y, 2.5, 0, Math.PI * 2)
      ctx.fill()
    }
  }
}

function drawTrend(
  ctx: CanvasRenderingContext2D,
  cssWidth: number,
  cssHeight: number,
): void {
  const left = 40
  const right = 12
  const top = 12
  const bottom = 26
  const width = Math.max(cssWidth - left - right, 10)
  const height = Math.max(cssHeight - top - bottom, 10)

  let max = 0
  for (const value of props.income) {
    max = Math.max(max, Number(value))
  }
  for (const value of props.expense) {
    max = Math.max(max, Number(value))
  }
  if (max <= 0) {
    max = 1
  }

  ctx.font = '11px -apple-system, BlinkMacSystemFont, sans-serif'
  ctx.fillStyle = INK
  ctx.textAlign = 'right'
  for (let i = 0; i <= 3; i += 1) {
    const y = top + (height / 3) * i
    ctx.strokeStyle = GRID
    ctx.lineWidth = 1
    ctx.beginPath()
    ctx.moveTo(left, y)
    ctx.lineTo(left + width, y)
    ctx.stroke()
    ctx.fillText(compact(max - (max / 3) * i), left - 6, y + 4)
  }

  drawLine(ctx, props.income, max, left, top, width, height, INCOME)
  drawLine(ctx, props.expense, max, left, top, width, height, EXPENSE)

  // x 轴刻度抽稀：最多显示 7 个标签。
  const labels = props.labels
  const stride = labels.length > 7 ? Math.ceil(labels.length / 7) : 1
  ctx.textAlign = 'center'
  ctx.fillStyle = INK
  for (let i = 0; i < labels.length; i += stride) {
    const x = left + (labels.length > 1 ? (width / (labels.length - 1)) * i : 0)
    ctx.fillText(labels[i], x, top + height + 16)
  }
}

function drawShare(
  ctx: CanvasRenderingContext2D,
  cssWidth: number,
  cssHeight: number,
): void {
  const shares = props.shares
  const cx = cssWidth / 2
  const cy = cssHeight / 2
  const outer = Math.min(cssWidth, cssHeight) / 2 - 8
  const inner = outer * 0.58

  if (shares.length === 0) {
    ctx.strokeStyle = GRID
    ctx.lineWidth = 14
    ctx.beginPath()
    ctx.arc(cx, cy, (outer + inner) / 2, 0, Math.PI * 2)
    ctx.stroke()
    return
  }

  let start = -Math.PI / 2
  for (let i = 0; i < shares.length; i += 1) {
    const ratio = shares[i].ratio
    if (ratio <= 0) {
      continue
    }
    const end = start + ratio * Math.PI * 2
    ctx.beginPath()
    ctx.fillStyle = PIE_COLORS[i % PIE_COLORS.length]
    ctx.arc(cx, cy, outer, start, end)
    ctx.arc(cx, cy, inner, end, start, true)
    ctx.closePath()
    ctx.fill()
    start = end
  }

  ctx.textAlign = 'center'
  ctx.fillStyle = '#1a1d1f'
  ctx.font = '600 13px -apple-system, BlinkMacSystemFont, sans-serif'
  ctx.fillText('支出占比', cx, cy - 2)
  ctx.fillStyle = INK
  ctx.font = '11px -apple-system, BlinkMacSystemFont, sans-serif'
  ctx.fillText(`${shares.length} 个分类`, cx, cy + 15)
}

function draw(): void {
  const canvas = canvasRef.value
  if (canvas === null || typeof canvas.getContext !== 'function') {
    return
  }
  const ctx = canvas.getContext('2d')
  if (ctx === null) {
    return
  }
  const cssWidth = canvas.clientWidth > 0 ? canvas.clientWidth : 600
  const cssHeight = props.height
  const dpr =
    typeof window !== 'undefined' && window.devicePixelRatio > 0
      ? window.devicePixelRatio
      : 1
  canvas.width = Math.round(cssWidth * dpr)
  canvas.height = Math.round(cssHeight * dpr)
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  ctx.clearRect(0, 0, cssWidth, cssHeight)
  if (props.kind === 'trend') {
    drawTrend(ctx, cssWidth, cssHeight)
  } else {
    drawShare(ctx, cssWidth, cssHeight)
  }
}

onMounted(() => {
  draw()
})

watch(
  () => [props.kind, props.labels, props.income, props.expense, props.shares, props.height],
  () => {
    draw()
  },
  { deep: true },
)

// 供父组件在容器尺寸变化（PC / 移动断点切换）时手动重绘。
defineExpose({ redraw: draw })
</script>

<template>
  <div class="chart">
    <canvas ref="canvasRef" class="chart__canvas" :style="{ height: `${height}px` }" />
  </div>
</template>

<style scoped>
.chart {
  width: 100%;
}

.chart__canvas {
  display: block;
  width: 100%;
}
</style>
