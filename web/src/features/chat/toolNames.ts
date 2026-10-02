import i18n from '@/i18n'

/** A tool's name: chat:tools.names.<tool id> in the catalog, or its id for a tool the catalog doesn't name. */
export function toolDisplayName(toolId: string): string {
  const key = `chat:tools.names.${toolId}`
  if (!toolId || !i18n.exists(key)) return toolId
  // An id such as "git" names a group of tools in the catalog, not a tool.
  const name: unknown = i18n.t(key, { returnObjects: true })
  return typeof name === 'string' ? name : toolId
}
