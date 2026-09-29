import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { EmptyState } from '@/components/ui/EmptyState'
import { api, ApiError } from '@/lib/api'
import { subscribeEvents } from '@/lib/events'
import { readScreenshotLaunch } from '@/lib/screenshotMode'
import type { Automation, AutomationDetail, AutomationInput, AutomationRun } from '@/types/api'
import { AutomationForm } from './AutomationForm'
import { formatWhen, notificationLabel, scheduleLabel } from './parseRequest'

const screenshotSentence =
  'Every morning at 8:00 AM, check this product and tell me if the price is below $500.'

export function AutomationsPage() {
  const queryClient = useQueryClient()
  const [selectedID, setSelectedID] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)
  const [editing, setEditing] = useState(false)
  const [formError, setFormError] = useState('')

  const listQuery = useQuery({
    queryKey: ['automations'],
    queryFn: () => api.listAutomations(),
  })
  const profilesQuery = useQuery({
    queryKey: ['profiles'],
    queryFn: () => api.getProfiles(),
  })
  const toolsQuery = useQuery({
    queryKey: ['tools'],
    queryFn: () => api.listTools(),
  })
  const detailQuery = useQuery({
    queryKey: ['automation', selectedID],
    queryFn: () => api.getAutomation(selectedID ?? ''),
    enabled: Boolean(selectedID),
  })

  useEffect(() => {
    return subscribeEvents({
      onEvent: (event) => {
        if (!event.type.startsWith('automation.')) return
        void queryClient.invalidateQueries({ queryKey: ['automations'] })
        void queryClient.invalidateQueries({ queryKey: ['automation'] })
      },
    })
  }, [queryClient])

  const items = listQuery.data ?? []
  const detail = detailQuery.data
  const profiles = profilesQuery.data ?? []
  const tools = toolsQuery.data ?? []

  useEffect(() => {
    if (!readScreenshotLaunch()?.enabled) return
    const compose = new URLSearchParams(window.location.search).get('compose') === '1'
    if (compose) {
      setCreating(true)
      return
    }
    if (!selectedID && items[0]) setSelectedID(items[0].id)
  }, [items, selectedID])

  function refresh() {
    void queryClient.invalidateQueries({ queryKey: ['automations'] })
    void queryClient.invalidateQueries({ queryKey: ['automation'] })
  }

  const save = useMutation({
    mutationFn: async (input: AutomationInput) => {
      if (editing && selectedID) return api.updateAutomation(selectedID, input)
      return api.createAutomation(input)
    },
    onSuccess: (saved) => {
      setFormError('')
      setCreating(false)
      setEditing(false)
      if (saved?.id) setSelectedID(saved.id)
      refresh()
    },
    onError: (error) => setFormError(error instanceof ApiError ? error.message : 'Could not save the automation.'),
  })

  const pause = useMutation({
    mutationFn: (item: Automation) => (item.enabled ? api.pauseAutomation(item.id) : api.resumeAutomation(item.id)),
    onSuccess: refresh,
  })
  const remove = useMutation({
    mutationFn: (id: string) => api.deleteAutomation(id),
    onSuccess: () => {
      setSelectedID(null)
      setEditing(false)
      refresh()
    },
  })
  const runNow = useMutation({
    mutationFn: (id: string) => api.runAutomation(id),
    onSuccess: refresh,
  })

  return (
    <div className="page-fill gap-4 overflow-y-auto p-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="font-display text-2xl font-semibold text-ink">Automations</h1>
          <p className="mt-1 max-w-2xl text-sm text-ink-muted">
            Scheduled prompts run in the background while this window is closed. The daemon starts a model when one is needed and lets it unload afterward.
          </p>
        </div>
        <button
          type="button"
          className="btn-primary px-3 py-1.5 text-xs"
          onClick={() => {
            setCreating(true)
            setEditing(false)
            setSelectedID(null)
            setFormError('')
          }}
        >
          New automation
        </button>
      </div>
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(20rem,32rem)]">
        <section>
          {listQuery.isLoading && <p className="text-sm text-ink-muted">Loading automations…</p>}
          {listQuery.isError && <p className="text-sm text-danger">Automations could not be loaded.</p>}
          {!listQuery.isLoading && items.length === 0 && !creating && (
            <EmptyState
              title="No automations yet"
              description="Describe a recurring check, such as a morning price or a Friday release summary, and Yggdrasil will run it on that schedule."
            />
          )}
          <ul className="space-y-2">
            {items.map((item) => (
              <li key={item.id}>
                <button
                  type="button"
                  className={['card w-full text-left', selectedID === item.id ? 'selectable-active' : ''].filter(Boolean).join(' ')}
                  onClick={() => {
                    setSelectedID(item.id)
                    setCreating(false)
                    setEditing(false)
                    setFormError('')
                  }}
                >
                  <div className="flex items-start justify-between gap-3">
                    <p className="font-medium text-ink">{item.name}</p>
                    <span className={item.enabled ? 'text-xs text-success' : 'text-xs text-ink-faint'}>
                      {item.enabled ? 'Enabled' : 'Paused'}
                    </span>
                  </div>
                  <p className="mt-1 text-xs text-ink-muted">{scheduleLabel(item.schedule)}</p>
                  <p className="mt-2 text-xs text-ink-faint">
                    {statusLabel(item.last_status)}
                    {item.last_run_at ? ` · ${formatWhen(item.last_run_at, item.schedule.time_zone)}` : ''}
                    {' · Next '}
                    {formatWhen(item.next_run_at, item.schedule.time_zone)}
                  </p>
                  {item.last_result && <p className="mt-2 line-clamp-2 text-sm text-ink-muted">{item.last_result}</p>}
                </button>
              </li>
            ))}
          </ul>
        </section>
        <aside>
          {creating || editing ? (
            <AutomationForm
              key={editing ? selectedID ?? 'edit' : 'new'}
              profiles={profiles}
              tools={tools}
              initial={editing ? detail : null}
              seedDescription={
                creating && new URLSearchParams(window.location.search).get('compose') === '1'
                  ? screenshotSentence
                  : ''
              }
              pending={save.isPending}
              error={formError}
              onCancel={() => {
                setCreating(false)
                setEditing(false)
                setFormError('')
              }}
              onSubmit={(input) => save.mutate(input)}
            />
          ) : detail ? (
            <Detail
              detail={detail}
              running={runNow.isPending}
              runError={runNow.error instanceof Error ? runNow.error.message : ''}
              onRun={() => runNow.mutate(detail.id)}
              onToggle={() => pause.mutate(detail)}
              onEdit={() => {
                setEditing(true)
                setFormError('')
              }}
              onDelete={() => {
                if (window.confirm(`Delete “${detail.name}”? Its history is removed too.`)) {
                  remove.mutate(detail.id)
                }
              }}
            />
          ) : (
            <div className="card text-sm text-ink-muted">Select an automation to see its history, or create one.</div>
          )}
        </aside>
      </div>
    </div>
  )
}

function Detail({
  detail,
  running,
  runError,
  onRun,
  onToggle,
  onEdit,
  onDelete,
}: {
  detail: AutomationDetail
  running: boolean
  runError: string
  onRun: () => void
  onToggle: () => void
  onEdit: () => void
  onDelete: () => void
}) {
  const zone = detail.schedule.time_zone
  return (
    <div className="card space-y-4">
      <div>
        <div className="flex items-start justify-between gap-3">
          <h2 className="font-display text-lg font-semibold text-ink">{detail.name}</h2>
          <span className={detail.enabled ? 'text-xs text-success' : 'text-xs text-ink-faint'}>
            {detail.enabled ? 'Enabled' : 'Paused'}
          </span>
        </div>
        <p className="mt-1 text-sm text-ink-muted">{scheduleLabel(detail.schedule)}</p>
        <p className="text-sm text-ink-muted">{notificationLabel(detail.notification)}</p>
        <p className="mt-2 text-xs text-ink-faint">
          Next {formatWhen(detail.next_run_at, zone)}
          {detail.last_error ? ` · ${detail.last_error}` : ''}
        </p>
      </div>
      <p className="whitespace-pre-wrap text-sm text-ink">{detail.prompt}</p>
      <div className="flex flex-wrap gap-2">
        <button type="button" className="btn-primary px-3 py-1.5 text-xs" disabled={running} onClick={onRun}>
          {running ? 'Running…' : 'Run now'}
        </button>
        <button type="button" className="btn-secondary px-3 py-1.5 text-xs" onClick={onToggle}>
          {detail.enabled ? 'Pause' : 'Resume'}
        </button>
        <button type="button" className="btn-secondary px-3 py-1.5 text-xs" onClick={onEdit}>
          Edit
        </button>
        <button type="button" className="btn-danger px-3 py-1.5 text-xs" onClick={onDelete}>
          Delete
        </button>
      </div>
      {runError && <p className="text-sm text-danger">{runError}</p>}
      <div>
        <h3 className="text-sm font-medium text-ink">History</h3>
        {detail.history.length === 0 ? (
          <p className="mt-2 text-sm text-ink-muted">This automation has not run yet.</p>
        ) : (
          <ul className="mt-2 space-y-3">
            {detail.history.map((run) => (
              <HistoryRow key={run.id} run={run} zone={zone} />
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}

function HistoryRow({ run, zone }: { run: AutomationRun; zone: string }) {
  return (
    <li className="rounded-lg bg-raised/50 p-3">
      <div className="flex items-start justify-between gap-3">
        <p className="text-sm font-medium text-ink">{statusLabel(run.status)}</p>
        <p className="text-xs text-ink-faint">{run.notification_sent ? 'Notified' : 'Not notified'}</p>
      </div>
      <p className="mt-1 text-xs text-ink-faint">
        Scheduled {formatWhen(run.occurrence_at, zone)}
        {run.started_at ? ` · Started ${formatWhen(run.started_at, zone)}` : ''}
        {run.finished_at ? ` · Finished ${formatWhen(run.finished_at, zone)}` : ''}
      </p>
      {(run.model_id || run.node_id) && (
        <p className="mt-1 text-xs text-ink-faint">
          {[run.model_id, run.node_id].filter(Boolean).join(' · ')}
          {run.attempt > 1 ? ` · Attempt ${run.attempt}` : ''}
        </p>
      )}
      {run.result && <p className="mt-2 whitespace-pre-wrap text-sm text-ink-muted">{run.result}</p>}
      {run.error && <p className="mt-2 text-sm text-danger">{run.error}</p>}
    </li>
  )
}

function statusLabel(status: string | undefined): string {
  switch (status) {
    case 'succeeded':
      return 'Succeeded'
    case 'failed':
      return 'Failed'
    case 'retrying':
      return 'Retrying'
    case 'running':
    case 'claimed':
      return 'Running'
    default:
      return 'Not run yet'
  }
}
