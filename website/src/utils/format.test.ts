import { describe, expect, it } from 'vitest'
import { formatBytes, formatBytesPerSec, formatNumber, formatTimestamp } from './format'

describe('formatBytes', () => {
  it('handles zero and negative values', () => {
    expect(formatBytes(0)).toBe('0 B')
    expect(formatBytes(-100)).toBe('0 B')
    expect(formatBytes(Number.NaN)).toBe('0 B')
  })

  it('formats each unit', () => {
    expect(formatBytes(512)).toBe('512.00 B')
    expect(formatBytes(1024)).toBe('1.00 KB')
    expect(formatBytes(1536)).toBe('1.50 KB')
    expect(formatBytes(1024 * 1024)).toBe('1.00 MB')
    expect(formatBytes(1024 ** 3)).toBe('1.00 GB')
  })

  it('clamps huge values to the largest unit', () => {
    expect(formatBytes(1024 ** 8)).toBe('1073741824.00 PB')
  })
})

describe('formatBytesPerSec', () => {
  it('handles zero and negative values', () => {
    expect(formatBytesPerSec(0)).toBe('0 bps')
    expect(formatBytesPerSec(-1)).toBe('0 bps')
  })

  it('converts bytes to bits', () => {
    expect(formatBytesPerSec(50)).toBe('400.0 bps')
    expect(formatBytesPerSec(1024 * 100)).toBe('819.20 Kbps')
    expect(formatBytesPerSec(1024 * 1024)).toBe('8.39 Mbps')
    expect(formatBytesPerSec(1024 * 1024 * 200)).toBe('1.68 Gbps')
    expect(formatBytesPerSec(1024 * 1024 * 1024)).toBe('8.59 Gbps')
  })
})

describe('formatTimestamp', () => {
  it('returns dash for empty or invalid input', () => {
    expect(formatTimestamp(undefined)).toBe('-')
    expect(formatTimestamp(null)).toBe('-')
    expect(formatTimestamp('not-a-date')).toBe('-')
  })

  it('formats a valid ISO timestamp', () => {
    const result = formatTimestamp('2026-01-02T03:04:05Z')
    // 不校验具体时区展示，只确认输出包含日期与时间成分
    expect(result).toMatch(/2026/)
    expect(result).toMatch(/\d{2}:\d{2}:\d{2}/)
  })
})

describe('formatNumber', () => {
  it('adds thousand separators', () => {
    expect(formatNumber(1234567)).toBe('1,234,567')
  })
})
