import { Link } from 'react-router-dom'
import { formatBytes } from '@/lib/format'
import type { AIProfile, HardwareInventory, RunningModelView } from '@/types/api'
import { Ratatoskr } from '@/components/ui/Ratatoskr'

export function RunningTab({
  running,
  profiles,
  localHardware,
  tightModelIds,
  onStop,
}: {
  running: RunningModelView[]
  profiles: AIProfile[]
  localHardware: HardwareInventory | null
  tightModelIds?: Set<string>
  onStop: (modelId: string, instanceId: string, nodeId: string) => void
}) {
  if (running.length === 0) {
    return (
      <div className="flex items-center gap-4">
        <Ratatoskr state="sleep" size={96} />
        <p className="text-sm text-ink-muted">
          No models are loaded right now. Chat will start one automatically when you need it.
        </p>
      </div>
    )
  }

  const totalMem = running.reduce((s, r) => s + (r.memory_bytes ?? 0), 0)
  const hostMem =
    localHardware?.accelerators?.[0]?.unified_memory_bytes ||
    localHardware?.accelerators?.[0]?.dedicated_vram_bytes ||
    localHardware?.memory?.total_bytes ||
    0

  return (
    <div className="space-y-4">
      <p className="text-sm text-ink-muted">
        Active models
        {totalMem > 0 ? ` · ~${formatBytes(totalMem)} in use` : ''}
        {hostMem > 0 ? ` of ${formatBytes(hostMem)} on this computer` : ''}
      </p>
      <ul className="grid gap-4 lg:grid-cols-2">
        {running.map((item) => {
          const usedBy =
            item.used_by_profiles && item.used_by_profiles.length > 0
              ? item.used_by_profiles
              : profiles
                  .filter((p) => p.roles?.some((r) => r.model_id === item.model_id))
                  .map((p) => p.name)

          return (
            <li key={item.instance_id} className="card min-w-0 space-y-3">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <h3 className="font-display text-lg font-semibold text-ink">
                    {item.display_name}
                  </h3>
                  {tightModelIds?.has(item.model_id) ? (
                    <p
                      className="mt-1 text-xs text-ink-muted"
                      title="Uses most of this computer's available memory and may be less stable."
                    >
                      Tight fit
                    </p>
                  ) : null}
                  <p className="mt-1 text-sm text-success">
                    Running on {item.node_name}
                    {item.accelerator ? ` · ${item.accelerator}` : ''}
                  </p>
                </div>
                <button
                  type="button"
                  className="btn-secondary shrink-0 px-3 py-1.5 text-xs"
                  onClick={() => onStop(item.model_id, item.instance_id, item.node_id)}
                >
                  Stop
                </button>
              </div>

              <dl className="grid grid-cols-2 gap-3 text-sm">
                <div>
                  <dt className="label-caps">Memory</dt>
                  <dd className="mt-0.5 tabular-nums text-ink">
                    {item.memory_bytes
                      ? hostMem > 0
                        ? `${formatBytes(item.memory_bytes)} / ${formatBytes(hostMem)}`
                        : formatBytes(item.memory_bytes)
                      : 'Memory usage unavailable'}
                  </dd>
                </div>
                <div>
                  <dt className="label-caps">Speed</dt>
                  <dd className="mt-0.5 tabular-nums text-ink">
                    {item.speed_tok_per_sec
                      ? `${Math.round(item.speed_tok_per_sec)} tok/s`
                      : 'Speed unavailable'}
                  </dd>
                </div>
              </dl>

              {usedBy.length > 0 && (
                <p className="text-xs text-ink-faint">Used by: {usedBy.join(', ')}</p>
              )}

              <div className="flex flex-wrap gap-2">
                <Link to="/chat" className="btn-primary px-3 py-1.5 text-xs">
                  Open Chat
                </Link>
              </div>
            </li>
          )
        })}
      </ul>
    </div>
  )
}
