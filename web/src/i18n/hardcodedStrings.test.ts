// @vitest-environment node
import { ESLint } from 'eslint'
import { describe, expect, it } from 'vitest'

// Runs the project's ESLint config on small components, so the check for
// hard-coded text (§5) keeps catching what it should and leaving code alone.
const eslint = new ESLint({ cwd: new URL('../..', import.meta.url).pathname })

async function literals(code: string, filePath = 'src/features/example/Example.tsx'): Promise<string[]> {
  const [result] = await eslint.lintText(code, { filePath })
  return result.messages.filter((m) => m.ruleId === 'i18next/no-literal-string').map((m) => m.message)
}

describe('hard-coded text check', () => {
  it('flags text people read', async () => {
    expect(await literals(`export const Example = () => <button>New automation</button>`)).toHaveLength(1)
    expect(await literals(`export const Example = () => <input placeholder="Search models" />`)).toHaveLength(1)
    expect(await literals(`export const Example = () => <button aria-label="Close">×</button>`)).toHaveLength(1)
    expect(await literals(`export const Example = ({ busy }: { busy: boolean }) => <p>{busy ? 'Saving…' : 'Save'}</p>`)).toHaveLength(2)
  })

  it('leaves code and names alone', async () => {
    const code = `
      import { useTranslation } from 'react-i18next'
      export function Example({ ok }: { ok: boolean }) {
        const { t } = useTranslation()
        return (
          <a href="/settings" className="btn-primary px-3 py-1.5" data-state={ok ? 'ready' : 'idle'} type="button">
            {t('nav.settings')} · MCP 3/4 <span>Mimir</span>
            <input placeholder="https://example.com/api" />
          </a>
        )
      }`
    expect(await literals(code)).toEqual([])
  })

  it('does not check tests', async () => {
    expect(await literals(`export const Example = () => <p>Hello there</p>`, 'src/features/example/Example.test.tsx')).toEqual([])
  })
})
