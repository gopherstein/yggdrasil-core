import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type {
  NotificationCategory,
  NotificationDestination,
  NotificationDestinationInput,
  NotificationSeverity,
  QuietHours,
} from '@/types/api'
import { CATEGORY_LABEL } from './notificationLabels'

const KEY = ['notification-destinations'] as const

const SEVERITIES: { value: '' | NotificationSeverity; label: string }[] = [
  { value: '', label: 'Everything' },
  { value: 'success', label: 'Successes, warnings, and errors' },
  { value: 'warning', label: 'Warnings and errors' },
  { value: 'error', label: 'Errors only' },
]

function errorText(err: unknown): string {
  return err instanceof Error ? err.message : 'Something went wrong.'
}

/**
 * Where notifications go besides the app (Gjallarhorn §12–13, §24–26): email
 * through your own SMTP server, and signed webhooks, each with the
 * categories and severities it receives, and quiet hours. Passwords and
 * signing secrets stay on this computer and are never shown again.
 */
export function NotificationDestinations() {
  const query = useQuery({ queryKey: KEY, queryFn: () => api.listNotificationDestinations(), retry: false })
  const [adding, setAdding] = useState<'' | NotificationDestination['kind']>('')
  const destinations = query.data ?? []
  return (
    <section className="card space-y-4">
      <div>
        <h2 className="section-title">Email, push, and webhooks</h2>
        <p className="mt-1 text-sm text-ink-muted">
          Send notifications to your email, to your phone or computer with ntfy, or to a service you run. Every
          notification is still in the bell. A delivery
          that fails is tried again after 1, 5, and 30 minutes, without running the task again.
        </p>
      </div>
      {query.isError && <p className="text-sm text-danger">{errorText(query.error)}</p>}
      {destinations.map((d) => (
        <DestinationRow key={d.id} destination={d} />
      ))}
      {adding ? (
        <DestinationForm kind={adding} onDone={() => setAdding('')} />
      ) : (
        <div className="flex flex-wrap gap-2">
          <button type="button" className="btn-secondary px-3 py-1.5 text-xs" onClick={() => setAdding('email')}>
            Add email
          </button>
          <button type="button" className="btn-secondary px-3 py-1.5 text-xs" onClick={() => setAdding('ntfy')}>
            Add push (ntfy)
          </button>
          <button type="button" className="btn-secondary px-3 py-1.5 text-xs" onClick={() => setAdding('webhook')}>
            Add webhook
          </button>
        </div>
      )}
      <QuietHoursForm />
    </section>
  )
}

const KIND_LABEL: Record<NotificationDestination['kind'], string> = { email: 'Email', webhook: 'Webhook', ntfy: 'Push' }

function summary(d: NotificationDestination): string {
  const where =
    d.kind === 'email'
      ? (d.email?.to ?? []).join(', ')
      : d.kind === 'ntfy'
        ? `${d.ntfy?.topic ?? ''} on ${d.ntfy?.server.replace(/^https?:\/\//, '') ?? ''}${d.ntfy?.content === 'private' ? ' (private)' : ''}`
        : (d.webhook?.url ?? '')
  const what = d.categories?.length ? d.categories.map((c) => CATEGORY_LABEL[c]).join(', ') : 'All categories'
  const level = SEVERITIES.find((s) => s.value === (d.min_severity ?? ''))?.label ?? 'Everything'
  return `${where} · ${what} · ${level}`
}

function DestinationRow({ destination: d }: { destination: NotificationDestination }) {
  const queryClient = useQueryClient()
  const [editing, setEditing] = useState(false)
  const [result, setResult] = useState<{ ok: boolean; text: string } | null>(null)
  const [secret, setSecret] = useState('')
  const refresh = () => queryClient.invalidateQueries({ queryKey: KEY })

  const test = useMutation({
    mutationFn: () => api.testNotificationDestination(d.id),
    onSuccess: (r) => setResult(r.ok ? { ok: true, text: 'Sent. Check that it arrived.' } : { ok: false, text: r.error ?? 'Not sent.' }),
    onError: (err) => setResult({ ok: false, text: errorText(err) }),
  })
  const toggle = useMutation({
    mutationFn: () => api.updateNotificationDestination(d.id, { enabled: !d.enabled }),
    onSettled: () => void refresh(),
  })
  const remove = useMutation({ mutationFn: () => api.deleteNotificationDestination(d.id), onSettled: () => void refresh() })
  const rotate = useMutation({
    mutationFn: () => api.rotateNotificationSecret(d.id),
    onSuccess: (r) => setSecret(r?.secret ?? ''),
  })

  if (editing) return <DestinationForm kind={d.kind} existing={d} onDone={() => setEditing(false)} />
  return (
    <div className="rounded-lg border border-line/60 p-3">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="text-sm font-medium text-ink">
            {d.name} <span className="text-xs font-normal text-ink-faint">{KIND_LABEL[d.kind]}</span>
            {!d.enabled && <span className="ml-1 text-xs font-normal text-ink-faint">· Off</span>}
          </p>
          <p className="mt-0.5 break-all text-xs text-ink-muted">{summary(d)}</p>
        </div>
        <div className="flex shrink-0 flex-wrap gap-2">
          <button type="button" className="btn-secondary px-3 py-1.5 text-xs" disabled={test.isPending} onClick={() => test.mutate()}>
            {test.isPending ? 'Sending…' : 'Test'}
          </button>
          <button type="button" className="btn-secondary px-3 py-1.5 text-xs" onClick={() => setEditing(true)}>
            Change
          </button>
          <button type="button" className="btn-secondary px-3 py-1.5 text-xs" disabled={toggle.isPending} onClick={() => toggle.mutate()}>
            {d.enabled ? 'Turn off' : 'Turn on'}
          </button>
          {d.kind === 'webhook' && (
            <button
              type="button"
              className="btn-secondary px-3 py-1.5 text-xs"
              disabled={rotate.isPending}
              onClick={() => {
                if (window.confirm('Make a new signing secret? The old one stops working at once.')) rotate.mutate()
              }}
            >
              New secret
            </button>
          )}
          <button
            type="button"
            className="btn-secondary px-3 py-1.5 text-xs"
            disabled={remove.isPending}
            onClick={() => {
              if (window.confirm(`Remove “${d.name}”? Notifications waiting for it are not sent.`)) remove.mutate()
            }}
          >
            Remove
          </button>
        </div>
      </div>
      {result && <p className={['mt-2 text-xs', result.ok ? 'text-success' : 'text-danger'].join(' ')}>{result.text}</p>}
      {secret && <SecretOnce secret={secret} />}
    </div>
  )
}

function SecretOnce({ secret }: { secret: string }) {
  return (
    <div className="mt-2 rounded-md bg-info/10 px-2.5 py-2 text-xs leading-relaxed text-ink">
      <p>Signing secret, shown this once. Use it to check the Yggdrasil-Signature header:</p>
      <code className="mt-1 block break-all font-mono">{secret}</code>
    </div>
  )
}

function DestinationForm({
  kind,
  existing,
  onDone,
}: {
  kind: NotificationDestination['kind']
  existing?: NotificationDestination
  onDone: () => void
}) {
  const queryClient = useQueryClient()
  const [name, setName] = useState(existing?.name ?? { email: 'My email', webhook: 'My webhook', ntfy: 'My phone' }[kind])
  const [server, setServer] = useState(existing?.ntfy?.server ?? 'https://ntfy.sh')
  const [topic, setTopic] = useState(existing?.ntfy?.topic ?? randomTopic())
  const [content, setContent] = useState<'' | 'full' | 'private'>(existing?.ntfy?.content ?? '')
  const [openUrl, setOpenUrl] = useState(existing?.ntfy?.open_url ?? '')
  const [url, setUrl] = useState(existing?.webhook?.url ?? '')
  const [host, setHost] = useState(existing?.email?.host ?? '')
  const [port, setPort] = useState(String(existing?.email?.port ?? ''))
  const [tls, setTls] = useState(existing?.email?.tls ?? 'starttls')
  const [username, setUsername] = useState(existing?.email?.username ?? '')
  const [password, setPassword] = useState('')
  const [from, setFrom] = useState(existing?.email?.from ?? '')
  const [to, setTo] = useState((existing?.email?.to ?? []).join(', '))
  const [categories, setCategories] = useState<NotificationCategory[]>(existing?.categories ?? [])
  const [minSeverity, setMinSeverity] = useState<'' | NotificationSeverity>(existing?.min_severity ?? '')
  const [error, setError] = useState('')
  const [secret, setSecret] = useState('')

  const save = useMutation({
    mutationFn: async () => {
      const input: NotificationDestinationInput = { name, categories, min_severity: minSeverity }
      if (kind === 'webhook') input.webhook = { url: url.trim() }
      else if (kind === 'ntfy') {
        input.ntfy = { server: server.trim(), topic: topic.trim(), content: content || undefined, open_url: openUrl.trim() || undefined }
        if (password) input.password = password
      } else {
        input.email = {
          host: host.trim(),
          port: Number(port) || 0,
          tls,
          username: username.trim() || undefined,
          from: from.trim(),
          to: to
            .split(',')
            .map((s) => s.trim())
            .filter(Boolean),
        }
        if (password) input.password = password
      }
      if (existing) {
        await api.updateNotificationDestination(existing.id, input)
        return ''
      }
      const created = await api.createNotificationDestination({ ...input, kind })
      return created?.secret ?? ''
    },
    onSuccess: (newSecret) => {
      void queryClient.invalidateQueries({ queryKey: KEY })
      if (newSecret) setSecret(newSecret)
      else onDone()
    },
    onError: (err) => setError(errorText(err)),
  })

  if (secret) {
    return (
      <div className="rounded-lg border border-line/60 p-3">
        <p className="text-sm font-medium text-ink">{name} added</p>
        <SecretOnce secret={secret} />
        <button type="button" className="btn-primary mt-3 px-3 py-1.5 text-xs" onClick={onDone}>
          Done
        </button>
      </div>
    )
  }

  const field = (label: string, value: string, set: (v: string) => void, props: Record<string, unknown> = {}) => (
    <label className="block text-sm">
      <span className="text-ink-muted">{label}</span>
      <input className="field mt-1 w-full" value={value} onChange={(e) => set(e.target.value)} spellCheck={false} {...props} />
    </label>
  )

  return (
    <form
      className="space-y-3 rounded-lg border border-line/60 p-3"
      onSubmit={(e) => {
        e.preventDefault()
        setError('')
        save.mutate()
      }}
    >
      {field('Name', name, setName)}
      {kind === 'ntfy' ? (
        <>
          <p className="rounded-md bg-info/10 px-2.5 py-2 text-xs leading-relaxed text-ink">
            Install the ntfy app (Android, iPhone, or ntfy.sh in a browser) and subscribe to the topic below. ntfy is
            free and open source; you can also run your own ntfy server.
          </p>
          <div className="grid gap-3 sm:grid-cols-2">
            {field('ntfy server', server, setServer, { placeholder: 'https://ntfy.sh' })}
            {field('Topic', topic, setTopic)}
          </div>
          {field('Access token (optional)', password, setPassword, {
            type: 'password',
            autoComplete: 'off',
            placeholder: existing?.has_secret ? 'Stored. Leave blank to keep it.' : 'Only if your topic needs one',
          })}
          <label className="block text-sm">
            <span className="text-ink-muted">What to send</span>
            <select className="field mt-1 w-full" value={content} onChange={(e) => setContent(e.target.value as typeof content)}>
              <option value="">Automatic (private on ntfy.sh, full on your own server)</option>
              <option value="full">Title and text</option>
              <option value="private">Only “You have a new Yggdrasil notification”</option>
            </select>
          </label>
          {field('Open notifications at', openUrl, setOpenUrl, { placeholder: 'http://192.168.1.10:7331' })}
          <p className="text-xs text-ink-faint">
            On ntfy.sh anyone who knows the topic can read it, so keep the random topic or send only the private notice.
          </p>
        </>
      ) : kind === 'webhook' ? (
        <>
          {field('Address', url, setUrl, { placeholder: 'https://example.com/yggdrasil' })}
          <p className="text-xs text-ink-faint">
            HTTPS, or HTTP on this computer or your local network. Each request is signed; the secret is shown once
            after you add it.
          </p>
        </>
      ) : (
        <>
          <div className="grid gap-3 sm:grid-cols-[1fr_7rem_9rem]">
            {field('SMTP server', host, setHost, { placeholder: 'smtp.example.com' })}
            {field('Port', port, setPort, { placeholder: tls === 'tls' ? '465' : '587', inputMode: 'numeric' })}
            <label className="block text-sm">
              <span className="text-ink-muted">Security</span>
              <select className="field mt-1 w-full" value={tls} onChange={(e) => setTls(e.target.value as typeof tls)}>
                <option value="starttls">STARTTLS</option>
                <option value="tls">TLS</option>
                <option value="none">None (this computer only)</option>
              </select>
            </label>
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            {field('Username', username, setUsername, { autoComplete: 'off' })}
            {field('Password', password, setPassword, {
              type: 'password',
              autoComplete: 'new-password',
              placeholder: existing?.has_secret ? 'Stored. Leave blank to keep it.' : '',
            })}
          </div>
          {field('From', from, setFrom, { placeholder: 'Yggdrasil <ygg@example.com>' })}
          {field('To', to, setTo, { placeholder: 'you@example.com, someone@example.com' })}
        </>
      )}
      <fieldset>
        <legend className="text-sm text-ink-muted">Categories (none checked: all)</legend>
        <div className="mt-1 flex flex-wrap gap-x-4 gap-y-1">
          {(Object.keys(CATEGORY_LABEL) as NotificationCategory[]).map((c) => (
            <label key={c} className="flex items-center gap-1.5 text-xs text-ink">
              <input
                type="checkbox"
                checked={categories.includes(c)}
                onChange={(e) => setCategories((cur) => (e.target.checked ? [...cur, c] : cur.filter((x) => x !== c)))}
              />
              {CATEGORY_LABEL[c]}
            </label>
          ))}
        </div>
      </fieldset>
      <label className="block text-sm">
        <span className="text-ink-muted">Send</span>
        <select className="field mt-1 w-full" value={minSeverity} onChange={(e) => setMinSeverity(e.target.value as typeof minSeverity)}>
          {SEVERITIES.map((s) => (
            <option key={s.value} value={s.value}>
              {s.label}
            </option>
          ))}
        </select>
      </label>
      {error && <p className="text-xs text-danger">{error}</p>}
      <div className="flex gap-2">
        <button type="submit" className="btn-primary px-3 py-1.5 text-xs" disabled={save.isPending}>
          {save.isPending ? 'Saving…' : existing ? 'Save' : 'Add'}
        </button>
        <button type="button" className="btn-secondary px-3 py-1.5 text-xs" onClick={onDone}>
          Cancel
        </button>
      </div>
    </form>
  )
}

const localZone = (() => {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
  } catch {
    return 'UTC'
  }
})()

function QuietHoursForm() {
  const queryClient = useQueryClient()
  const query = useQuery({ queryKey: ['quiet-hours'], queryFn: () => api.getQuietHours(), retry: false })
  const [q, setQ] = useState<QuietHours | null>(null)
  const [error, setError] = useState('')
  useEffect(() => {
    if (query.data && !q) setQ({ ...query.data, time_zone: query.data.enabled ? query.data.time_zone : localZone })
  }, [query.data, q])
  const save = useMutation({
    mutationFn: (next: QuietHours) => api.setQuietHours(next),
    onSuccess: () => {
      setError('')
      void queryClient.invalidateQueries({ queryKey: ['quiet-hours'] })
    },
    onError: (err) => setError(errorText(err)),
  })
  if (!q) return null
  const update = (patch: Partial<QuietHours>) => {
    const next = { ...q, ...patch }
    setQ(next)
    save.mutate(next)
  }
  return (
    <div className="space-y-2 border-t border-line/60 pt-4">
      <label className="flex items-center justify-between gap-3">
        <span>
          <span className="block text-sm font-medium text-ink">Quiet hours</span>
          <span className="block text-xs text-ink-muted">
            Desktop notices, email, push, and webhooks wait until quiet hours end. They are in the bell at once.
          </span>
        </span>
        <input type="checkbox" checked={q.enabled} onChange={(e) => update({ enabled: e.target.checked })} aria-label="Quiet hours" />
      </label>
      {q.enabled && (
        <div className="grid gap-3 sm:grid-cols-[7rem_7rem_1fr]">
          <label className="block text-sm">
            <span className="text-ink-muted">From</span>
            <input className="field mt-1 w-full" type="time" value={q.start} onChange={(e) => update({ start: e.target.value })} />
          </label>
          <label className="block text-sm">
            <span className="text-ink-muted">Until</span>
            <input className="field mt-1 w-full" type="time" value={q.end} onChange={(e) => update({ end: e.target.value })} />
          </label>
          <label className="block text-sm">
            <span className="text-ink-muted">During quiet hours</span>
            <select className="field mt-1 w-full" value={q.allow} onChange={(e) => update({ allow: e.target.value as QuietHours['allow'] })}>
              <option value="errors">Still send errors</option>
              <option value="nothing">Hold everything</option>
            </select>
          </label>
          <p className="text-xs text-ink-faint sm:col-span-3">Time zone: {q.time_zone}</p>
        </div>
      )}
      {error && <p className="text-xs text-danger">{error}</p>}
    </div>
  )
}

/** A topic that is hard to guess, for the public ntfy server. */
function randomTopic(): string {
  const bytes = new Uint8Array(8)
  crypto.getRandomValues(bytes)
  return `yggdrasil-${Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')}`
}
