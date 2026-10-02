import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import i18n, { applyLanguage } from '@/i18n'
import { AnswerDetails } from './AnswerDetails'
import { explainError } from './friendlyError'
import { roleLabel } from './runRoles'
import { toolDisplayName } from './toolNames'

afterEach(async () => {
  await applyLanguage('en')
  localStorage.clear()
})

describe('chat text from the catalog', () => {
  it('names tools by id, and leaves ids the catalog lacks as they are', () => {
    expect(toolDisplayName('internet.search')).toBe('Web search')
    expect(toolDisplayName('terminal')).toBe('Terminal')
    expect(toolDisplayName('git.commit')).toBe('Git commit')
    // "git" is a group of tools in the catalog, not a tool's name.
    expect(toolDisplayName('git')).toBe('git')
    expect(toolDisplayName('weather.lookup')).toBe('weather.lookup')
  })

  it('uses plural forms', () => {
    expect(i18n.t('chat:send.summarizeFile', { count: 1 })).toBe('Summarize this file.')
    expect(i18n.t('chat:send.summarizeFile', { count: 3 })).toBe('Summarize these files.')
    expect(i18n.t('chat:tools.used', { count: 1 })).toBe('Used 1 tool')
    expect(i18n.t('chat:tools.used', { count: 2 })).toBe('Used 2 tools')
    expect(i18n.t('chat:status.foundPassagesMore', { count: 1, source: 'Notes', more: 2 })).toBe(
      'Found 1 passage in Notes and 2 more…',
    )
  })

  it('names run roles', () => {
    expect(roleLabel('assistant')).toBe('Answer')
    expect(roleLabel('worker:2')).toBe('Worker 2')
    expect(roleLabel('planner')).toBe('Planner')
  })

  it('shows errors and answer details in the App language', async () => {
    await applyLanguage('en-XA')
    expect(explainError('failed to fetch').title).toMatch(/^\[!! .* !!\]$/)
    render(<AnswerDetails meta={{ steps: [{ kind: 'search', text: 'Looked it up' }, { kind: 'file', text: 'Wrote it' }] }} />)
    expect(screen.getByRole('button', { expanded: false }).textContent).toMatch(/\[!! .*2.* !!\]/)
  })
})
