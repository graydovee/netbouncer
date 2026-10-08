import { lazy, Suspense } from 'react'
import type { ComponentProps } from 'react'
import type { EChartsCoreOption } from 'echarts/core'
export type EChartsOption = EChartsCoreOption
const Chart = lazy(() => import('./EChartImpl').then((module) => ({ default: module.EChart })))
export const EChart = (props: ComponentProps<typeof Chart>) => <Suspense fallback={<div role="status" aria-label="图表加载中" style={{ height: props.height ?? 300 }} />}><Chart {...props} /></Suspense>
