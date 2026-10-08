import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { RealmKicker } from '@/components/ui/Realm'
import { Toggle } from '@/components/ui/Toggle'
import { useShareBase } from '@/features/people/shareBase'
import { PortalPreview } from '@/features/portal/PortalPreview'
import { availableLanguages, languages } from '@/i18n'
import { api, ApiError } from '@/lib/api'
import type { Portal, PortalBranding, PortalInput } from '@/types/api'
import { slugFrom } from './slug'

function errorText(err: unknown): string {
  return err instanceof ApiError ? err.message : err ? String(err) : ''
}

function usePrefersDark(): boolean {
  return typeof window !== 'undefined' && window.matchMedia ? window.matchMedia('(prefers-color-scheme: dark)').matches : true
}

/** The address visitors open, with a copy button. */
function PortalLink({ slug }: { slug: string }) {
  const { t } = useTranslation('portals')
  const { base, reachable } = useShareBase()
  const [copied, setCopied] = useState(false)
  const url = `${base}/p/${slug}`
  return (
    <div className="space-y-1">
      <div className="flex min-w-0 items-stretch gap-2">
        <code className="field min-w-0 flex-1 truncate py-1.5 font-mono text-xs">{url}</code>
        <button
          type="button"
          className="btn-secondary btn-sm shrink-0"
          onClick={() => void navigator.clipboard?.writeText(url).then(() => setCopied(true))}
        >
          {copied ? t('link.copied') : t('link.copy')}
        </button>
      </div>
      {!reachable ? (
        <p className="text-xs text-warning">
          {t('link.lanOff')}{' '}
          <Link to="/api-access" className="text-primary hover:underline">
            {t('link.lanOffAction')}
          </Link>
        </p>
      ) : null}
    </div>
  )
}

function AddPortal({ onAdded }: { onAdded: (p: Portal) => void }) {
  const { t } = useTranslation('portals')
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [slugTouched, setSlugTouched] = useState(false)
  const [access, setAccess] = useState<'passcode' | 'open'>('passcode')
  const [passcode, setPasscode] = useState('')
  const add = useMutation({
    mutationFn: () => api.createPortal({ name: name.trim(), slug, access, ...(access === 'passcode' ? { passcode } : {}) }),
    onSuccess: (p) => {
      setName('')
      setSlug('')
      setSlugTouched(false)
      setPasscode('')
      void queryClient.invalidateQueries({ queryKey: ['portals'] })
      if (p) onAdded(p)
    },
  })
  return (
    <section className="card space-y-3" aria-labelledby="add-portal">
      <h2 id="add-portal" className="section-title">
        {t('add.title')}
      </h2>
      <p className="text-sm text-ink-muted">{t('add.body')}</p>
      <form
        className="grid gap-3 sm:grid-cols-2"
        onSubmit={(e) => {
          e.preventDefault()
          add.mutate()
        }}
      >
        <label className="space-y-1 text-sm">
          <span className="text-ink-muted">{t('field.name')}</span>
          <input
            className="field w-full"
            value={name}
            maxLength={80}
            onChange={(e) => {
              setName(e.target.value)
              if (!slugTouched) setSlug(slugFrom(e.target.value))
            }}
          />
        </label>
        <label className="space-y-1 text-sm">
          <span className="text-ink-muted">{t('field.slug')}</span>
          <input
            className="field w-full font-mono"
            value={slug}
            maxLength={40}
            onChange={(e) => {
              setSlugTouched(true)
              setSlug(e.target.value.toLowerCase())
            }}
          />
        </label>
        <label className="space-y-1 text-sm">
          <span className="text-ink-muted">{t('field.access')}</span>
          <select className="field w-full" value={access} onChange={(e) => setAccess(e.target.value as 'passcode' | 'open')}>
            <option value="passcode">{t('access.passcode')}</option>
            <option value="open">{t('access.open')}</option>
          </select>
        </label>
        {access === 'passcode' ? (
          <label className="space-y-1 text-sm">
            <span className="text-ink-muted">{t('field.passcode')}</span>
            <input className="field w-full" value={passcode} autoComplete="off" onChange={(e) => setPasscode(e.target.value)} />
          </label>
        ) : (
          <p className="self-end text-xs text-ink-faint">{t('access.openHint')}</p>
        )}
        {add.error ? (
          <p className="text-sm text-danger sm:col-span-2" role="alert">
            {errorText(add.error)}
          </p>
        ) : null}
        <div className="sm:col-span-2">
          <button
            type="submit"
            className="btn-primary"
            disabled={add.isPending || !name.trim() || slug.length < 2 || (access === 'passcode' && passcode.trim().length < 6)}
          >
            {t('add.submit')}
          </button>
        </div>
      </form>
    </section>
  )
}

/** Branding as the editor holds it: every field a string. */
type BrandingDraft = Required<Omit<PortalBranding, 'prompts' | 'theme'>> & { theme: string; prompts: string }

function toDraft(b: PortalBranding): BrandingDraft {
  return {
    title: b.title ?? '',
    logo_url: b.logo_url ?? '',
    accent: b.accent ?? '',
    background: b.background ?? '',
    theme: b.theme ?? 'system',
    welcome: b.welcome ?? '',
    prompts: (b.prompts ?? []).join('\n'),
    footer: b.footer ?? '',
  }
}

function fromDraft(d: BrandingDraft): PortalBranding {
  const out: PortalBranding = { theme: d.theme as PortalBranding['theme'] }
  for (const key of ['title', 'logo_url', 'accent', 'background', 'welcome', 'footer'] as const) {
    const v = d[key].trim()
    if (v) out[key] = v
  }
  const lines = d.prompts
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean)
    .slice(0, 6)
  if (lines.length) out.prompts = lines
  return out
}

function PortalEditor({ portal, onClose }: { portal: Portal; onClose: () => void }) {
  const { t } = useTranslation('portals')
  const queryClient = useQueryClient()
  const prefersDark = usePrefersDark()
  const profiles = useQuery({ queryKey: ['profiles'], queryFn: () => api.getProfiles(), retry: false })
  const [name, setName] = useState(portal.name)
  const [slug, setSlug] = useState(portal.slug)
  const [profileId, setProfileId] = useState(portal.profile_id)
  const [tools, setTools] = useState(portal.tools)
  const [memory, setMemory] = useState(portal.memory)
  const [language, setLanguage] = useState(portal.language)
  const [access, setAccess] = useState(portal.access)
  const [passcode, setPasscode] = useState('')
  const [draft, setDraft] = useState(() => toDraft(portal.branding ?? {}))
  const [saved, setSaved] = useState(false)
  const branding = useMemo(() => fromDraft(draft), [draft])
  const set = (key: keyof BrandingDraft) => (value: string) => {
    setSaved(false)
    setDraft((d) => ({ ...d, [key]: value }))
  }

  const save = useMutation({
    mutationFn: () => {
      const input: PortalInput = { name: name.trim(), slug, profile_id: profileId, tools, memory, language, access, branding }
      if (passcode.trim()) input.passcode = passcode.trim()
      return api.updatePortal(portal.id, input)
    },
    onSuccess: () => {
      setPasscode('')
      setSaved(true)
      void queryClient.invalidateQueries({ queryKey: ['portals'] })
    },
  })
  useEffect(() => setSaved(false), [name, slug, profileId, tools, memory, language, access, passcode])

  const choices = languages.filter((l) => availableLanguages.includes(l.code))
  const field = 'space-y-1 text-sm'
  const label = 'text-ink-muted'

  return (
    <section className="card space-y-4" aria-labelledby="edit-portal">
      <div className="flex items-start justify-between gap-3">
        <h2 id="edit-portal" className="section-title">
          {t('edit.title', { name: portal.name })}
        </h2>
        <button type="button" className="btn-secondary btn-sm" onClick={onClose}>
          {t('edit.close')}
        </button>
      </div>
      <form
        className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_18rem]"
        onSubmit={(e) => {
          e.preventDefault()
          save.mutate()
        }}
      >
        <div className="min-w-0 space-y-5">
          <fieldset className="grid gap-3 sm:grid-cols-2">
            <legend className="label-caps mb-2">{t('edit.answers')}</legend>
            <label className={field}>
              <span className={label}>{t('field.name')}</span>
              <input className="field w-full" value={name} maxLength={80} onChange={(e) => setName(e.target.value)} />
            </label>
            <label className={field}>
              <span className={label}>{t('field.slug')}</span>
              <input className="field w-full font-mono" value={slug} maxLength={40} onChange={(e) => setSlug(e.target.value.toLowerCase())} />
            </label>
            <label className={field}>
              <span className={label}>{t('field.profile')}</span>
              <select className="field w-full" value={profileId} onChange={(e) => setProfileId(e.target.value)}>
                <option value="">{t('field.defaultProfile')}</option>
                {(profiles.data ?? []).map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </select>
            </label>
            <label className={field}>
              <span className={label}>{t('field.tools')}</span>
              <select className="field w-full" value={tools} onChange={(e) => setTools(e.target.value as Portal['tools'])}>
                <option value="none">{t('tools.none')}</option>
                <option value="read_only">{t('tools.read_only')}</option>
                <option value="profile">{t('tools.profile')}</option>
              </select>
            </label>
            <label className={field}>
              <span className={label}>{t('field.language')}</span>
              <select className="field w-full" value={language} onChange={(e) => setLanguage(e.target.value)}>
                <option value="">{t('field.followVisitor')}</option>
                {choices.map((l) => (
                  <option key={l.code} value={l.code} lang={l.code}>
                    {l.name}
                  </option>
                ))}
              </select>
            </label>
            <div className="flex items-center justify-between gap-3 self-end rounded-lg border border-line/60 px-3 py-2">
              <span className="text-sm text-ink">{t('field.memory')}</span>
              <Toggle label={t('field.memory')} checked={memory} onChange={() => setMemory(!memory)} />
            </div>
          </fieldset>

          <fieldset className="grid gap-3 sm:grid-cols-2">
            <legend className="label-caps mb-2">{t('edit.access')}</legend>
            <label className={field}>
              <span className={label}>{t('field.access')}</span>
              <select className="field w-full" value={access} onChange={(e) => setAccess(e.target.value as Portal['access'])}>
                <option value="passcode">{t('access.passcode')}</option>
                <option value="open">{t('access.open')}</option>
              </select>
            </label>
            {access === 'passcode' ? (
              <label className={field}>
                <span className={label}>{portal.has_passcode ? t('field.newPasscode') : t('field.passcode')}</span>
                <input
                  className="field w-full"
                  value={passcode}
                  autoComplete="off"
                  placeholder={portal.has_passcode ? t('field.keepPasscode') : ''}
                  onChange={(e) => setPasscode(e.target.value)}
                />
              </label>
            ) : (
              <p className="self-end text-xs text-ink-faint">{t('access.openHint')}</p>
            )}
          </fieldset>

          <fieldset className="grid gap-3 sm:grid-cols-2">
            <legend className="label-caps mb-2">{t('edit.looks')}</legend>
            <label className={field}>
              <span className={label}>{t('brand.title')}</span>
              <input className="field w-full" value={draft.title} maxLength={80} placeholder={name} onChange={(e) => set('title')(e.target.value)} />
            </label>
            <label className={field}>
              <span className={label}>{t('brand.logo')}</span>
              <input className="field w-full" value={draft.logo_url} placeholder="https://" onChange={(e) => set('logo_url')(e.target.value)} />
            </label>
            {(['accent', 'background'] as const).map((key) => (
              <label key={key} className={field}>
                <span className={label}>{t(`brand.${key}`)}</span>
                <span className="flex gap-2">
                  <input
                    type="color"
                    className="h-9 w-10 shrink-0 cursor-pointer rounded border border-line bg-transparent"
                    aria-label={t(`brand.${key}`)}
                    value={/^#[0-9a-f]{6}$/i.test(draft[key]) ? draft[key] : key === 'accent' ? '#4fd1c5' : '#0b0f14'}
                    onChange={(e) => set(key)(e.target.value)}
                  />
                  <input
                    className="field min-w-0 flex-1 font-mono"
                    value={draft[key]}
                    placeholder={t('brand.themeColour')}
                    onChange={(e) => set(key)(e.target.value)}
                  />
                </span>
              </label>
            ))}
            <label className={field}>
              <span className={label}>{t('brand.theme')}</span>
              <select className="field w-full" value={draft.theme} onChange={(e) => set('theme')(e.target.value)}>
                <option value="system">{t('theme.system')}</option>
                <option value="light">{t('theme.light')}</option>
                <option value="dark">{t('theme.dark')}</option>
              </select>
            </label>
            <label className={`${field} sm:col-span-2`}>
              <span className={label}>{t('brand.welcome')}</span>
              <textarea className="field w-full" rows={2} value={draft.welcome} onChange={(e) => set('welcome')(e.target.value)} />
            </label>
            <label className={field}>
              <span className={label}>{t('brand.prompts')}</span>
              <textarea className="field w-full" rows={3} value={draft.prompts} onChange={(e) => set('prompts')(e.target.value)} />
            </label>
            <label className={field}>
              <span className={label}>{t('brand.footer')}</span>
              <textarea className="field w-full" rows={3} value={draft.footer} onChange={(e) => set('footer')(e.target.value)} />
            </label>
          </fieldset>

          {save.error ? (
            <p className="text-sm text-danger" role="alert">
              {errorText(save.error)}
            </p>
          ) : null}
          <div className="flex items-center gap-3">
            <button type="submit" className="btn-primary" disabled={save.isPending || !name.trim() || slug.length < 2}>
              {t('edit.save')}
            </button>
            {saved ? (
              <span className="text-sm text-success" role="status">
                {t('edit.saved')}
              </span>
            ) : null}
          </div>
        </div>
        <div className="space-y-2 lg:sticky lg:top-4 lg:self-start">
          <p className="label-caps">{t('edit.preview')}</p>
          <PortalPreview name={name} branding={branding} prefersDark={prefersDark} />
        </div>
      </form>
    </section>
  )
}

function PortalRow({ portal, onEdit }: { portal: Portal; onEdit: () => void }) {
  const { t } = useTranslation('portals')
  const queryClient = useQueryClient()
  const change = useMutation({
    mutationFn: (enabled: boolean) => api.updatePortal(portal.id, { enabled }),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['portals'] }),
  })
  const remove = useMutation({
    mutationFn: () => api.deletePortal(portal.id),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['portals'] }),
  })
  return (
    <li className="space-y-2 py-3">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
        <div className="min-w-0 flex-1">
          <p className="font-medium text-ink">
            {portal.name}
            {!portal.enabled ? <span className="status-chip ms-2 bg-ink/10 text-ink-muted">{t('off')}</span> : null}
          </p>
          <p className="text-xs text-ink-muted">
            {portal.access === 'open' ? t('access.open') : t('access.passcode')} · {t(`tools.${portal.tools}`)}
          </p>
        </div>
        <Toggle label={t('enabled', { name: portal.name })} checked={portal.enabled} disabled={change.isPending} onChange={() => change.mutate(!portal.enabled)} />
        <button type="button" className="btn-secondary btn-sm" onClick={onEdit}>
          {t('edit.open')}
        </button>
        <button
          type="button"
          className="btn-secondary btn-sm"
          disabled={remove.isPending}
          onClick={() => {
            if (window.confirm(t('remove.confirm', { name: portal.name }))) remove.mutate()
          }}
        >
          {t('remove.button')}
        </button>
      </div>
      {portal.enabled ? <PortalLink slug={portal.slug} /> : null}
      {change.error || remove.error ? <p className="text-xs text-danger">{errorText(change.error ?? remove.error)}</p> : null}
    </li>
  )
}

/**
 * Administer → Portals (#205): branded chat pages for the people an Admin
 * serves. Add one, choose how it answers and who may use it, see how it
 * looks while editing, copy its link, and turn it off at once.
 */
export function PortalsPage() {
  const { t } = useTranslation('portals')
  const portals = useQuery({ queryKey: ['portals'], queryFn: () => api.listPortals() })
  const [editing, setEditing] = useState<string | null>(null)
  const list = portals.data ?? []
  const current = list.find((p) => p.id === editing)

  return (
    <div className="mx-auto w-full max-w-5xl min-w-0 space-y-6">
      <header className="page-header">
        <RealmKicker />
        <h1 className="page-title">{t('nav.portals', { ns: 'common' })}</h1>
        <p className="page-subtitle">{t('subtitle')}</p>
      </header>

      {current ? <PortalEditor key={current.id} portal={current} onClose={() => setEditing(null)} /> : null}

      <AddPortal onAdded={(p) => setEditing(p.id)} />

      <section className="card" aria-labelledby="all-portals">
        <h2 id="all-portals" className="section-title">
          {t('list.title')}
        </h2>
        {portals.isSuccess && list.length === 0 ? <p className="mt-2 text-sm text-ink-faint">{t('list.none')}</p> : null}
        <ul className="divide-y divide-line">
          {list.map((p) => (
            <PortalRow key={p.id} portal={p} onEdit={() => setEditing(p.id)} />
          ))}
        </ul>
        <p className="mt-3 text-xs text-ink-faint">{t('list.private')}</p>
      </section>
    </div>
  )
}
