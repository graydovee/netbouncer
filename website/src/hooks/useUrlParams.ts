import { useCallback } from 'react'
import { useSearchParams } from 'react-router-dom'

/**
 * 将列表页的分页/筛选/排序状态同步到 URL 查询串，
 * 刷新不丢状态、视图可分享。
 */
export function useUrlParams() {
  const [searchParams, setSearchParams] = useSearchParams()

  const getParam = (key: string, defaultValue = ''): string =>
    searchParams.get(key) ?? defaultValue

  const getNumberParam = (key: string, defaultValue: number): number => {
    const raw = searchParams.get(key)
    if (raw === null) {
      return defaultValue
    }
    const value = Number(raw)
    return Number.isFinite(value) ? value : defaultValue
  }

  const updateParams = useCallback(
    (updates: Record<string, string | number | undefined | null>) => {
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev)
          for (const [key, value] of Object.entries(updates)) {
            if (value === undefined || value === null || value === '') {
              next.delete(key)
            } else {
              next.set(key, String(value))
            }
          }
          return next
        },
        { replace: true },
      )
    },
    [setSearchParams],
  )

  return { searchParams, getParam, getNumberParam, updateParams }
}
