/** 格式化字节数，如 1536 -> "1.50 KB" */
export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return '0 B'
  }

  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)

  const value = bytes / Math.pow(1024, i)
  return `${value.toFixed(2)} ${units[i]}`
}

/** 格式化字节速率（输入为字节/秒，展示为比特率），如 1.5MB/s -> "12.00 Mbps" */
export function formatBytesPerSec(bytesPerSec: number): string {
  if (!Number.isFinite(bytesPerSec) || bytesPerSec <= 0) {
    return '0 bps'
  }

  const bps = bytesPerSec * 8
  if (bps < 1000) {
    return `${bps.toFixed(1)} bps`
  }
  if (bps < 1000 * 1000) {
    return `${(bps / 1000).toFixed(2)} Kbps`
  }
  if (bps < 1000 * 1000 * 1000) {
    return `${(bps / (1000 * 1000)).toFixed(2)} Mbps`
  }
  return `${(bps / (1000 * 1000 * 1000)).toFixed(2)} Gbps`
}

/** 格式化 ISO 时间戳为本地时间，空值显示 "-" */
export function formatTimestamp(timestamp?: string | null): string {
  if (!timestamp) {
    return '-'
  }
  const date = new Date(timestamp)
  if (Number.isNaN(date.getTime())) {
    return '-'
  }
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

/** 格式化整数为千分位字符串 */
export function formatNumber(value: number): string {
  return value.toLocaleString()
}
