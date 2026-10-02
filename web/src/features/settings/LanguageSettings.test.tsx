import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { applyLanguage } from '@/i18n'
import { api } from '@/lib/api'
import { useUIStore } from '@/stores/uiStore'
import type { SettingsView } from '@/types/api'
import { LanguageSettings } from './LanguageSettings'

vi.mock('@/lib/api', () => ({ api: { getSettings: vi.fn(), updateSettings: vi.fn() } }))

function renderIt() {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <LanguageSettings />
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.mocked(api.getSettings).mockResolvedValue({ ui_locale: '' } as SettingsView)
  vi.mocked(api.updateSettings).mockImplementation(async (patch) => ({ ui_locale: patch.ui_locale }) as SettingsView)
})

afterEach(async () => {
  useUIStore.getState().setAdvancedMode(false)
  await applyLanguage('en')
  localStorage.clear()
})

describe('LanguageSettings', () => {
  it('offers the system default, named in its language, and the languages with a catalog', async () => {
    renderIt()
    const select = await screen.findByRole('combobox', { name: 'App language' })
    await waitFor(() => expect(select).toBeEnabled())
    const options = [...select.querySelectorAll('option')].map((o) => o.textContent)
    expect(options).toEqual([
      'System default (English)',
      'English',
      'Deutsch',
      'Español',
      'Français',
      'Italiano',
      'Português (Brasil)',
      '日本語',
      '한국어',
      '简体中文',
      '繁體中文',
    ])
  })

  it('says when the chosen language is machine translated', async () => {
    vi.mocked(api.getSettings).mockResolvedValue({ ui_locale: 'de' } as SettingsView)
    renderIt()
    expect(await screen.findByText(/^Machine translated/)).toBeInTheDocument()
  })

  it('saves the choice on the daemon and shows it at once', async () => {
    useUIStore.getState().setAdvancedMode(true) // the pseudo-locale is offered in advanced mode
    renderIt()
    const select = await screen.findByRole('combobox', { name: 'App language' })
    await waitFor(() => expect(select).toBeEnabled())
    fireEvent.change(select, { target: { value: 'en-XA' } })
    await waitFor(() => expect(api.updateSettings).toHaveBeenCalledWith({ ui_locale: 'en-XA' }))
    // The section itself is now pseudo-localized.
    expect(await screen.findByText(/^\[!! .*Ļáá/)).toBeInTheDocument()
    expect(document.documentElement.lang).toBe('en-XA')
  })
})

describe('assistant language', () => {
  it('offers auto-detect, the App language, and languages by their own names', async () => {
    renderIt()
    const select = await screen.findByRole('combobox', { name: 'Assistant language' })
    await waitFor(() => expect(select).toBeEnabled())
    expect(select).toHaveValue(':auto')
    const options = [...select.querySelectorAll('option')].map((o) => o.textContent)
    expect(options.slice(0, 2)).toEqual(['Auto-detect: the language you write in', 'Same as the App language'])
    expect(options).toContain('Deutsch')
    expect(options).toContain('日本語')
  })

  it('saves a chosen language, or a mode', async () => {
    renderIt()
    const select = await screen.findByRole('combobox', { name: 'Assistant language' })
    await waitFor(() => expect(select).toBeEnabled())
    fireEvent.change(select, { target: { value: 'de' } })
    await waitFor(() =>
      expect(api.updateSettings).toHaveBeenCalledWith({ assistant_language_mode: 'language', assistant_language: 'de' }),
    )
    fireEvent.change(select, { target: { value: ':app' } })
    await waitFor(() => expect(api.updateSettings).toHaveBeenCalledWith({ assistant_language_mode: 'app' }))
  })

  it('shows the saved choice', async () => {
    vi.mocked(api.getSettings).mockResolvedValue({ ui_locale: '', assistant_language_mode: 'language', assistant_language: 'fr' } as SettingsView)
    renderIt()
    const select = await screen.findByRole('combobox', { name: 'Assistant language' })
    await waitFor(() => expect(select).toHaveValue('fr'))
  })
})
