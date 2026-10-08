import { afterEach, describe, expect, it, vi } from 'vitest'
import { trafficApi } from './traffic'

const response = (data: unknown) => new Response(JSON.stringify({ code: 200, data }), { status: 200, headers: { 'Content-Type': 'application/json' } })
afterEach(() => vi.unstubAllGlobals())

describe('traffic API v1 contract', () => {
  it('sends server pagination and returns global totals separately from page size', async () => {
    const fetch = vi.fn().mockResolvedValue(response({ items: [], total: 10000, snapshot_id: 123 }))
    vi.stubGlobal('fetch', fetch)
    const data = await trafficApi.get({ page: 2, page_size: 25, sort: 'total', order: 'desc', remote_ip: '2001:db8::1' })
    expect(data.total).toBe(10000)
    expect(data.items).toEqual([])
    const url = new URL(fetch.mock.calls[0][0], 'http://localhost')
    expect(url.searchParams.get('page')).toBe('2')
    expect(url.searchParams.get('page_size')).toBe('25')
    expect(url.searchParams.get('remote_ip')).toBe('2001:db8::1')
  })

  it('keeps actual bucket precision and gaps alongside historical points', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(response({ items: [{ ts: 60, bytes_in: 100 }], meta: { start: 60, end: 660, bucket: 600, freshness: 600, available_start: 60, gaps: [{ start: 120, end: 180, reason: 'unavailable' }] } })))
    const result = await trafficApi.history({ start: 60, end: 660, bucket: 300 })
    expect(result.meta.bucket).toBe(600)
    expect(result.meta.gaps).toHaveLength(1)
    expect(result.items[0].bytes_in).toBe(100)
  })

  it('propagates cancellation instead of reporting an obsolete request as a network failure', async () => {
    const controller = new AbortController()
    vi.stubGlobal('fetch', vi.fn((_url: string, options: RequestInit) => new Promise((_resolve, reject) => {
      expect(options.signal).toBe(controller.signal)
      options.signal?.addEventListener('abort', () => reject(new DOMException('Cancelled', 'AbortError')))
    })))
    const request = trafficApi.history({ start: 60, end: 660, bucket: 60 }, controller.signal)
    controller.abort()
    await expect(request).rejects.toHaveProperty('name', 'AbortError')
  })
})
