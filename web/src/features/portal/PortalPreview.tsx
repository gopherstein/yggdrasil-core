import { useTranslation } from 'react-i18next'
import type { PortalBranding } from '@/types/api'
import { prompts, safeLogo, text, theme, themeVars } from './branding'

/**
 * How a chat portal's page looks (#205), drawn from its branding as the
 * page draws it, for Administer → Portals to show while it's edited.
 */
export function PortalPreview({ name, branding, prefersDark }: { name: string; branding: PortalBranding; prefersDark: boolean }) {
  const { t } = useTranslation('portal')
  const title = text(branding.title, name || '…', 80)
  const logo = safeLogo(branding.logo_url)
  const suggestions = prompts(branding)
  return (
    <div
      data-theme={theme(branding, prefersDark)}
      style={themeVars(branding)}
      className="flex min-h-[18rem] flex-col overflow-hidden rounded-xl border border-line bg-canvas text-ink"
      aria-hidden
    >
      <div className="flex items-center gap-2 border-b border-line/60 px-3 py-2">
        {logo ? <img src={logo} alt="" className="h-6 w-6 rounded object-contain" /> : null}
        <span className="truncate font-display text-sm font-semibold">{title}</span>
      </div>
      <div className="flex flex-1 flex-col items-center justify-center gap-3 px-4 py-6 text-center">
        <p className="text-xs text-ink-muted">{text(branding.welcome, t('welcome', { name: name || '…' }), 1000)}</p>
        {suggestions.length > 0 ? (
          <div className="flex flex-wrap justify-center gap-1.5">
            {suggestions.map((s) => (
              <span key={s} className="rounded-md bg-ink/5 px-2 py-1 text-[11px] text-ink">
                {s}
              </span>
            ))}
          </div>
        ) : null}
      </div>
      <div className="flex items-center gap-2 border-t border-line/60 px-3 py-2">
        <span className="field flex-1 py-1 text-[11px] text-ink-faint">{t('message', { name: name || '…' })}</span>
        <span className="rounded-md bg-primary px-2 py-1 text-[11px] text-primary-fg">{t('send')}</span>
      </div>
      {text(branding.footer) ? <p className="border-t border-line/60 px-3 py-1.5 text-center text-[10px] text-ink-faint">{text(branding.footer)}</p> : null}
    </div>
  )
}
