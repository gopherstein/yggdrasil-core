import { useTranslation } from 'react-i18next'
import type { TopicsDraft } from './topicsDraft'

/**
 * A profile's topic controls (#345): what the assistant stays on, what it
 * never discusses, and the reply to anything else. They come first in
 * every turn, as the administrator's rules, and nothing people type or set
 * changes them.
 */
export function TopicsEditor({ value, onChange, disabled }: { value: TopicsDraft; onChange: (d: TopicsDraft) => void; disabled?: boolean }) {
  const { t } = useTranslation('profiles')
  const set = (key: keyof TopicsDraft) => (e: { target: { value: string } }) => onChange({ ...value, [key]: e.target.value })
  const staysOn = value.staysOn.trim()
  return (
    <div className="max-w-2xl space-y-4">
      <p className="text-sm text-ink-muted">{t('topics.intro')}</p>
      <label className="block space-y-1 text-sm">
        <span className="text-ink-muted">{t('topics.staysOn')}</span>
        <textarea
          className="field w-full"
          rows={2}
          maxLength={1000}
          disabled={disabled}
          placeholder={t('topics.staysOnExample')}
          value={value.staysOn}
          onChange={set('staysOn')}
        />
        <span className="block text-xs text-ink-faint">{t('topics.staysOnHint')}</span>
      </label>
      {staysOn ? (
        <>
          <div className="grid gap-4 sm:grid-cols-2">
            <label className="block space-y-1 text-sm">
              <span className="text-ink-muted">{t('topics.examples')}</span>
              <textarea className="field w-full" rows={4} disabled={disabled} value={value.examples} onChange={set('examples')} />
              <span className="block text-xs text-ink-faint">{t('topics.perLine')}</span>
            </label>
            <label className="block space-y-1 text-sm">
              <span className="text-ink-muted">{t('topics.never')}</span>
              <textarea className="field w-full" rows={4} disabled={disabled} value={value.never} onChange={set('never')} />
              <span className="block text-xs text-ink-faint">{t('topics.neverHint')}</span>
            </label>
          </div>
          <label className="block space-y-1 text-sm">
            <span className="text-ink-muted">{t('topics.reply')}</span>
            <textarea
              className="field w-full"
              rows={2}
              maxLength={500}
              disabled={disabled}
              placeholder={t('topics.replyDefault', { topic: staysOn })}
              value={value.reply}
              onChange={set('reply')}
            />
            <span className="block text-xs text-ink-faint">{t('topics.replyHint')}</span>
          </label>
          <p className="rounded-lg bg-ink/5 p-3 text-xs text-ink-muted">{t('topics.note')}</p>
        </>
      ) : null}
    </div>
  )
}
