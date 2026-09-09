import type { ApiResponse } from './types'

/** API 调用错误，携带 HTTP 状态码，message 为可直接展示的提示 */
export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

type UnauthorizedHandler = () => void

let unauthorizedHandler: UnauthorizedHandler | null = null

/** 注册 401 统一处理回调（由 AuthContext 在挂载时注册） */
export function setUnauthorizedHandler(handler: UnauthorizedHandler | null): void {
  unauthorizedHandler = handler
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  let response: Response
  try {
    response = await fetch(path, {
      ...options,
      headers: { 'Content-Type': 'application/json', ...(options.headers ?? {}) },
    })
  } catch {
    throw new ApiError(0, '网络请求失败，请检查后端服务是否可用')
  }

  // 会话过期/未登录：通知全局处理器（跳转登录），并抛出可识别的错误
  if (response.status === 401) {
    unauthorizedHandler?.()
    throw new ApiError(401, '未授权访问，请先登录')
  }

  let result: ApiResponse<T>
  try {
    result = (await response.json()) as ApiResponse<T>
  } catch {
    throw new ApiError(response.status, `请求失败 (HTTP ${response.status})`)
  }

  if (!response.ok || result.code !== 200) {
    throw new ApiError(result.code ?? response.status, result.message || `请求失败 (HTTP ${response.status})`)
  }

  return result.data
}

export function get<T>(path: string): Promise<T> {
  return request<T>(path)
}

export function post<T>(path: string, body?: unknown): Promise<T> {
  return request<T>(path, {
    method: 'POST',
    body: body === undefined ? undefined : JSON.stringify(body),
  })
}

export function put<T>(path: string, body: unknown): Promise<T> {
  return request<T>(path, { method: 'PUT', body: JSON.stringify(body) })
}

export function del<T>(path: string): Promise<T> {
  return request<T>(path, { method: 'DELETE' })
}

/** 将查询参数对象序列化为 URL 查询串，忽略空值 */
export function buildQuery(params: Record<string, string | number | undefined>): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') {
      search.set(key, String(value))
    }
  }
  const query = search.toString()
  return query ? `?${query}` : ''
}

/** 从错误对象中提取可展示的提示文案 */
export function errorMessage(error: unknown, fallback = '操作失败'): string {
  if (error instanceof ApiError) {
    return error.message
  }
  if (error instanceof Error) {
    return `${fallback}: ${error.message}`
  }
  return fallback
}
