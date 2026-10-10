import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { applyLanguage } from '@/i18n'
import { api } from '@/lib/api'
import { AddModelPanel } from './AddModelPanel'

vi.mock('@/lib/api', () => ({
  api: { importModelPath: vi.fn(), findModelsInOtherApps: vi.fn(), installModelFromURL: vi.fn() },
  uploadModel: vi.fn(),
}))

function renderIt() {
  const onAdded = vi.fn()
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <AddModelPanel onClose={vi.fn()} onAdded={onAdded} />
    </QueryClientProvider>,
  )
  return onAdded
}

describe('AddModelPanel (#467)', () => {
  beforeEach(async () => {
    await applyLanguage('en')
    vi.mocked(api.importModelPath).mockReset().mockResolvedValue({ model_id: 'qwen', status: 'installed', details: { name: 'Qwen3 8B' } })
    vi.mocked(api.findModelsInOtherApps).mockResolvedValue({
      models: [
        { app: 'ollama', path: '/o/blobs/sha256-1', name: 'qwen3-coder:30b', size_bytes: 18e9, parameters: '30B-A3B', quantization: 'Q4_K_M' },
        { app: 'lmstudio', path: '/l/a.gguf', name: 'Already', size_bytes: 1e9, model_id: 'already' },
      ],
    })
  })

  it('adds a file on this computer, used where it is', async () => {
    const onAdded = renderIt()
    fireEvent.change(screen.getByLabelText('Or the location of a file on this computer'), { target: { value: '/m/qwen.gguf' } })
    fireEvent.click(screen.getByLabelText(/Use it where it is/))
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    expect(await screen.findByText("Added Qwen3 8B. It's on the Installed tab.")).toBeInTheDocument()
    expect(api.importModelPath).toHaveBeenCalledWith({ path: '/m/qwen.gguf', in_place: true })
    expect(onAdded).toHaveBeenCalledWith('qwen')
  })

  it('lists models other apps downloaded and adds one in place', async () => {
    renderIt()
    fireEvent.click(screen.getByRole('tab', { name: 'From another app' }))
    expect(await screen.findByText('qwen3-coder:30b')).toBeInTheDocument()
    expect(screen.getByText(/Ollama · 30B-A3B · Q4_K_M/)).toBeInTheDocument()
    expect(screen.getByText('Added')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    await screen.findByText(/Added qwen3-coder:30b\./)
    expect(api.importModelPath).toHaveBeenCalledWith({ path: '/o/blobs/sha256-1', in_place: true, display_name: 'qwen3-coder:30b' })
  })
})
