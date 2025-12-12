import * as echarts from 'echarts/core'
import { LineChart, BarChart } from 'echarts/charts'
import {
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
  ToolboxComponent
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { use } from 'echarts/core'
import { defineNuxtPlugin } from 'nuxt/app'

// 注册必须的组件
use([
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
  ToolboxComponent,
  LineChart,
  BarChart,
  CanvasRenderer
])

export default defineNuxtPlugin(() => {
  return {
    provide: {
      echarts
    }
  }
})