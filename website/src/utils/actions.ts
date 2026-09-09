import type { IpNetAction } from '../api/types'

/** 动作的中文名称 */
const ACTION_LABELS: Record<IpNetAction, string> = {
  ban: '禁用',
  allow: '允许',
}

/** 动作对应的展示颜色 */
const ACTION_COLORS: Record<IpNetAction, 'error' | 'success'> = {
  ban: 'error',
  allow: 'success',
}

/** 获取动作中文名称，未知动作原样返回 */
export function actionLabel(action: string): string {
  return ACTION_LABELS[action as IpNetAction] ?? action
}

/** 获取动作展示颜色，未知动作返回 default */
export function actionColor(action: string): 'error' | 'success' | 'default' {
  return ACTION_COLORS[action as IpNetAction] ?? 'default'
}

/** 所有支持的规则动作（顺序与后端 /api/ip/action 一致） */
export const AVAILABLE_ACTIONS: IpNetAction[] = ['ban', 'allow']
