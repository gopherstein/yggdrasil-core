import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useLocation } from 'react-router-dom'
import { api } from '@/lib/api'
import { isDesktopShell } from '@/lib/desktopBridge'
import { reportAddress, reportMailLink, reportText } from '@/lib/report'
import { useDialog } from '@/lib/useDialog'

/**
 * Report this: a link beside an error that shows everything a report
 * would send, lets the person add what they were doing, and sends it by
 * email or copies it (#588).
 */
export function ReportProblem({ error, code, className }: { error: string; code?: string; className?: string }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  return (
    <>
      <button type="button" className={className ?? 'text-xs text-ink-faint hover:text-ink'} onClick={() => setOpen(true)}>
        {t('report.open')}
      </button>
      {open ? <ReportDialog error={error} code={code} onClose={() => setOpen(false)} /> : null}
    </>
  )
}

function ReportDialog({ error, code, onClose }: { error: string; code?: string; onClose: () => void }) {
  const { t } = useTranslation()
  const ref = useDialog(true, onClose)
  const { pathname } = useLocation()
  const [note, setNote] = useState('')
  const [copied, setCopied] = useState(false)
  const report = useQuery({
    queryKey: ['diagnostics', 'report'],
    queryFn: api.getReport,
    retry: false,
    staleTime: 60_000,
  })
  const problem = {
    error,
    code,
    where: pathname,
    app: isDesktopShell() ? 'desktop' : 'web',
  }
  const full = reportText(problem, note, report.data)
  const subject = `Toskar problem: ${code || pathname}`
  const mail = reportMailLink(subject, (n) => reportText(problem, note, report.data, n), report.data?.log?.length ?? 0)
  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center scrim p-4 sm:items-center" role="presentation" onClick={onClose}>
      <div
        ref={ref}
        className="card w-full max-w-lg space-y-3 shadow-panel"
        role="dialog"
        aria-modal="true"
        aria-labelledby="report-title"
        onClick={(event) => event.stopPropagation()}
      >
        <h2 id="report-title" className="font-display text-lg font-semibold text-ink">
          {t('report.title')}
        </h2>
        <p className="text-sm text-ink-muted">{t('report.body', { address: reportAddress })}</p>
        <label className="block text-sm font-medium text-ink">
          {t('report.noteLabel')}
          <textarea
            className="field mt-1 min-h-[4.5rem] w-full font-normal"
            value={note}
            placeholder={t('report.notePlaceholder')}
            data-autofocus
            onChange={(event) => setNote(event.target.value)}
          />
        </label>
        <div>
          <p className="text-xs font-medium text-ink-muted">{t('report.whatSends')}</p>
          <pre className="log-panel mt-1 max-h-56 overflow-auto whitespace-pre-wrap break-words text-xs">
            {report.isPending ? t('report.gathering') : full}
          </pre>
        </div>
        <div className="flex flex-wrap items-center justify-end gap-2">
          <button type="button" className="btn-secondary me-auto" onClick={onClose}>
            {t('report.cancel')}
          </button>
          <button
            type="button"
            className="btn-secondary"
            disabled={report.isPending}
            onClick={() => {
              void navigator.clipboard.writeText(full).then(() => setCopied(true))
            }}
          >
            {copied ? t('report.copied') : t('report.copy')}
          </button>
          <a
            href={mail}
            className={`btn-primary${report.isPending ? ' pointer-events-none opacity-60' : ''}`}
            aria-disabled={report.isPending}
          >
            {t('report.send')}
          </a>
        </div>
      </div>
    </div>
  )
}
