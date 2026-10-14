// A problem report's text and mail link (#588); components/ui/ReportProblem
// shows them.

import { scrub } from './scrub'
import type { DiagnosticsReport } from '@/types/api'

/** Where reports go until Toskar has its own report service (#588). */
export const reportAddress = 'info@yeix.io'

// A mail app takes a link of a few thousand characters at most; the
// oldest log lines go first when a report is longer.
const maxMailLink = 6000

/**
 * The report as plain text, in English for the people who read it: what
 * went wrong and where, what the person adds, and Core's report. The error
 * is scrubbed here; Core scrubs what it adds.
 */
export function reportText(
  problem: { error: string; code?: string; where: string; app: string },
  note: string,
  report?: DiagnosticsReport | null,
  logLines = report?.log?.length ?? 0,
): string {
  const lines = ['Toskar problem report', `Where: ${problem.where}`, `App: ${problem.app}`]
  if (problem.code) lines.push(`Code: ${problem.code}`)
  lines.push(`Error: ${scrub(problem.error) || 'none shown'}`)
  if (note.trim()) lines.push('', 'What I was doing:', scrub(note.trim()))
  if (report) {
    lines.push(
      '',
      `Toskar ${report.version}${report.commit ? ` (${report.commit.slice(0, 12)})` : ''}, contract ${report.contract}`,
      `Computer: ${report.os} ${report.arch}${report.cpu ? `, ${report.cpu}` : ''}${report.memory_gb ? `, ${report.memory_gb} GB` : ''}`,
    )
    if (report.gpus?.length) lines.push(`Graphics: ${report.gpus.join(', ')}`)
    const log = (report.log ?? []).slice(-logLines || undefined)
    if (logLines > 0 && log.length) lines.push('', 'Recent warnings and errors:', ...log)
  }
  return lines.join('\n')
}

/** A mail link with the report, shortened to what a mail app takes. */
export function reportMailLink(subject: string, build: (logLines: number) => string, logLines: number): string {
  for (let n = logLines; ; n--) {
    const link = `mailto:${reportAddress}?subject=${encodeURIComponent(subject)}&body=${encodeURIComponent(build(n))}`
    if (link.length <= maxMailLink || n <= 0) return link
  }
}
