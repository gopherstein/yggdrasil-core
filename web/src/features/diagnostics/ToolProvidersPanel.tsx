import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '@/lib/api'
import type { ToolProvider } from '@/types/api'

/** The tools shown; each is named at providers.tools.<id> in the catalog. */
const TOOLS = ['image.generate', 'image.edit', 'video.generate', 'speech.transcribe', 'speech.synthesize'] as const

const TONE: Record<ToolProvider['state'], string> = {
  healthy: 'text-success',
  installing: 'text-warning',
  failed: 'text-danger',
  unavailable: 'text-ink-faint',
}

/**
 * Which computer can run each heavy tool (Gungnir §16). A tool asked for
 * here runs on a computer that can, preferring one whose GPU does the work.
 */
export function ToolProvidersPanel() {
  const { t } = useTranslation('diagnostics')
  const query = useQuery({ queryKey: ['tools', 'providers'], queryFn: () => api.toolProviders(), retry: false, staleTime: 15_000 })
  const nodes = Array.isArray(query.data) ? query.data : []
  if (nodes.length === 0) return null
  return (
    <section className="card space-y-3">
      <div>
        <h2 className="section-title">{t('providers.title')}</h2>
        <p className="mt-1 text-sm text-ink-muted">{t('providers.description')}</p>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-left text-sm">
          <thead>
            <tr className="text-xs text-ink-faint">
              <th className="py-1.5 pr-3 font-medium">{t('providers.computer')}</th>
              {TOOLS.map((tool) => (
                <th key={tool} className="py-1.5 pr-3 font-medium">
                  {t(`providers.tools.${tool}`)}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {nodes.map((node) => (
              <tr key={node.node_id || node.name} className="border-t border-line/50 align-top">
                <td className="py-2 pr-3">
                  <span className="text-ink">{node.name || node.node_id}</span>
                  {node.local ? <span className="ml-1.5 text-xs text-ink-faint">{t('providers.thisComputer')}</span> : null}
                  {node.note ? <span className="block text-xs text-ink-faint">{node.note}</span> : null}
                </td>
                {TOOLS.map((tool) => {
                  const p = node.providers.find((x) => x.tool === tool)
                  if (!p) {
                    return (
                      <td key={tool} className="py-2 pr-3 text-ink-faint">
                        –
                      </td>
                    )
                  }
                  const state = p.state in TONE ? p.state : 'unavailable'
                  return (
                    <td key={tool} className="py-2 pr-3" title={p.reason || p.name}>
                      <span className={TONE[state]}>{t(`providers.states.${state}`)}</span>
                      {p.state === 'healthy' && p.accelerated ? (
                        <span className="ml-1.5 text-xs text-ink-faint">{t('providers.gpu')}</span>
                      ) : null}
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
