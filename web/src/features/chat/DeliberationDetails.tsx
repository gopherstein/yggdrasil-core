import { useTranslation } from 'react-i18next'
import { useState } from 'react'
import type { Deliberation, DeliberationDraft } from '@/types/api'

/** "drafter:2" is Draft 2; the turn's own answer is Draft 1. */
function draftNumber(role: string, index: number): number {
  const n = Number(role.split(':')[1])
  return Number.isFinite(n) && n > 0 ? n : index + 1
}

function where(d: { model_id?: string; node_name?: string }): string {
  return [d.model_id?.split('/').pop(), d.node_name].filter(Boolean).join(' · ')
}

function DraftRow({ draft, index }: { draft: DeliberationDraft; index: number }) {
  const { t } = useTranslation('chat')
  const [open, setOpen] = useState(false)
  const n = draftNumber(draft.role, index)
  const detail = where(draft)
  return (
    <li className="rounded-md border border-line/60 px-2.5 py-1.5">
      <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
        <span className="font-medium text-ink">{t('roles.drafter', { n })}</span>
        {draft.chosen ? <span className="rounded-full bg-primary/10 px-1.5 text-[10px] text-primary">{t('deliberation.kept')}</span> : null}
        {detail ? <span className="text-ink-faint">{detail}</span> : null}
      </div>
      {draft.failed ? (
        <p className="mt-0.5 text-warning">{t('deliberation.failed')}</p>
      ) : (
        <p className="mt-0.5">
          <span className="text-ink-faint">{t('deliberation.final')} </span>
          <span className="text-ink">{draft.final || t('deliberation.noFinal')}</span>
        </p>
      )}
      {draft.text ? (
        <>
          <button
            type="button"
            className="mt-0.5 underline-offset-2 hover:text-ink hover:underline"
            aria-expanded={open}
            onClick={() => setOpen((v) => !v)}
          >
            {open ? t('deliberation.hideDraft') : t('deliberation.showDraft')}
          </button>
          {open ? <p className="mt-1 max-h-64 overflow-y-auto whitespace-pre-wrap text-ink-muted">{draft.text}</p> : null}
        </>
      ) : null}
    </li>
  )
}

/**
 * The drafts Deliberate compared for an answer (#459): each draft's model,
 * computer, and final answer, the critiques when they disagreed, and the
 * judge that wrote the answer.
 */
export function DeliberationDetails({ deliberation }: { deliberation: Deliberation }) {
  const { t } = useTranslation('chat')
  const [open, setOpen] = useState(false)
  const drafts = deliberation.drafts ?? []
  if (drafts.length === 0) return null
  const critiques = (deliberation.critiques ?? []).filter((c) => !c.failed)
  const judge = deliberation.judge
  const summary = t(`deliberation.summary.${judge ? 'judged' : deliberation.outcome}`, { count: drafts.length })
  return (
    <div className="text-xs text-ink-muted">
      <button
        type="button"
        className="underline-offset-2 hover:text-ink hover:underline"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
      >
        {open ? '▾' : '▸'} {summary}
      </button>
      {open ? (
        <div className="mt-1.5 space-y-2">
          <ol className="space-y-1.5">
            {drafts.map((d, i) => (
              <DraftRow key={d.role + i} draft={d} index={i} />
            ))}
          </ol>
          {critiques.length > 0 ? (
            <div>
              <p className="label-caps mb-1 text-[10px]">{t('deliberation.critiques')}</p>
              <ul className="space-y-1.5">
                {critiques.map((c, i) => {
                  const points = [...(c.likely_errors ?? []), ...(c.disagreements ?? [])]
                  return (
                    <li key={i}>
                      <span className="text-ink">
                        {t('deliberation.critique', { draft: c.draft + 1, critic: draftNumber(c.critic, 0) })}
                      </span>
                      {points.length > 0 ? (
                        <ul className="mt-0.5 list-disc space-y-0.5 ps-4">
                          {points.map((p, j) => (
                            <li key={j}>{p}</li>
                          ))}
                        </ul>
                      ) : (
                        <span> {t('deliberation.noIssues')}</span>
                      )}
                    </li>
                  )
                })}
              </ul>
            </div>
          ) : null}
          {judge ? (
            <p>
              <span className="text-ink">{t('roles.judge')}</span>
              {where(judge) ? <span className="text-ink-faint"> · {where(judge)}</span> : null}
              <span> {t('deliberation.judgeWrote')}</span>
            </p>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}
