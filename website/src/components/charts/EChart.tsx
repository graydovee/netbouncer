import { useEffect, useRef } from 'react'
import * as echarts from 'echarts/core'
import { BarChart, LineChart } from 'echarts/charts'
import {
  DataZoomComponent,
  GridComponent,
  LegendComponent,
  TooltipComponent,
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { useColorMode } from '../../theme'

echarts.use([
  LineChart,
  BarChart,
  GridComponent,
  TooltipComponent,
  LegendComponent,
  DataZoomComponent,
  CanvasRenderer,
])

export type EChartsOption = echarts.EChartsCoreOption

interface EChartProps {
  option: EChartsOption
  height?: number | string
  /** 点击系列元素回调（参数为 ECharts 的 params 对象） */
  onClick?: (params: unknown) => void
}

/** ECharts 受控封装：跟随明暗主题重建实例，容器尺寸自适应 */
export const EChart = ({ option, height = 300, onClick }: EChartProps) => {
  const containerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<echarts.ECharts | null>(null)
  const onClickRef = useRef(onClick)
  onClickRef.current = onClick
  const { mode } = useColorMode()

  // 主题切换时重建实例
  useEffect(() => {
    const container = containerRef.current
    if (!container) {
      return
    }

    const chart = echarts.init(container, mode === 'dark' ? 'dark' : undefined)
    chartRef.current = chart

    const resizeObserver = new ResizeObserver(() => chart.resize())
    resizeObserver.observe(container)

    if (onClick) {
      chart.on('click', (params) => onClickRef.current?.(params))
    }

    return () => {
      resizeObserver.disconnect()
      chart.dispose()
      chartRef.current = null
    }
  }, [mode, onClick])

  useEffect(() => {
    chartRef.current?.setOption(
      { backgroundColor: 'transparent', ...option },
      { notMerge: true },
    )
  }, [option])

  return <div ref={containerRef} style={{ width: '100%', height }} />
}
