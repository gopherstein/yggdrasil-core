import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { ToolProvider } from '@/types/api'

const TOOLS: { id: string; label: string }[] = [
  { id: 'image.generate', label: 'Make images' },
  { id: 'image.edit', label: 'Edit images' },
  { id: 'speech.transcribe', label: 'Transcribe' },
  { id: 'speech.synthesize', label: 'Read aloud' },
]

const STATE: Record<ToolProvider['state'], { text: string; tone: string }> = {
  healthy: { text: 'Ready', tone: 'text-success' },
  installing: { text: 'Installing', tone: 'text-warning' },
  failed: { text: 'Failed', tone: 'text-danger' },
  unavailable: { text: 'Not set up', tone: 'text-ink-faint' },
}

/**
 * Which computer can run each heavy tool (Gungnir §16). A tool asked for
 * here runs on a computer that can, preferring one whose GPU does the work.
 */
export function ToolProvidersPanel() {
  const query = useQuery({ queryKey: ['tools', 'providers'], queryFn: () => api.toolProviders(), retry: false, staleTime: 15_000 })
  const nodes = Array.isArray(query.data) ? query.data : []
  if (nodes.length === 0) return null
  return (
    <section className="card space-y-3">
      <div>
        <h2 className="section-title">Tools on each computer</h2>
        <p className="mt-1 text-sm text-ink-muted">
          Images and speech run on whichever paired computer can run them, preferring one whose GPU does the work. Files
          are sent to it for the job and the result comes back to this chat.
        </p>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-left text-sm">
          <thead>
            <tr className="text-xs text-ink-faint">
              <th className="py-1.5 pr-3 font-medium">Computer</th>
              {TOOLS.map((t) => (
                <th key={t.id} className="py-1.5 pr-3 font-medium">
                  {t.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {nodes.map((node) => (
              <tr key={node.node_id || node.name} className="border-t border-line/50 align-top">
                <td className="py-2 pr-3">
                  <span className="text-ink">{node.name || node.node_id}</span>
                  {node.local ? <span className="ml-1.5 text-xs text-ink-faint">this computer</span> : null}
                  {node.note ? <span className="block text-xs text-ink-faint">{node.note}</span> : null}
                </td>
                {TOOLS.map((t) => {
                  const p = node.providers.find((x) => x.tool === t.id)
                  if (!p) {
                    return (
                      <td key={t.id} className="py-2 pr-3 text-ink-faint">
                        –
                      </td>
                    )
                  }
                  const s = STATE[p.state] ?? STATE.unavailable
                  return (
                    <td key={t.id} className="py-2 pr-3" title={p.reason || p.name}>
                      <span className={s.tone}>{s.text}</span>
                      {p.state === 'healthy' && p.accelerated ? <span className="ml-1.5 text-xs text-ink-faint">GPU</span> : null}
                    </td>
                  )
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  )
}
