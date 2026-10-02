import { formatGigabytes, formatSize } from '@/i18n/format'

/** A size in bytes in the UI's format, or a dash when there is none. */
export function formatBytes(bytes: number | undefined): string {
  if (bytes == null || bytes === 0) {
    return '—'
  }
  return formatSize(bytes)
}

export function bytesToGb(bytes: number | undefined): string {
  if (bytes == null) {
    return '—'
  }
  return formatGigabytes(bytes / 1024 ** 3)
}
