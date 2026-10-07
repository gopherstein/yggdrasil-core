import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { api } from '@/lib/api'
import type { ImageModel, ImageSetup } from '@/types/api'
import { ImageSetupCard } from './ImageSetup'

vi.mock('@/lib/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/api')>()
  return { ...actual, api: { ...actual.api, getMediaSetup: vi.fn(), startMediaSetup: vi.fn(), useMediaBuild: vi.fn(), cancelMediaSetup: vi.fn(), removeMediaModel: vi.fn() } }
})

const model = (over: Partial<ImageModel>): ImageModel => ({
  id: 'flux2-klein-4b', name: 'FLUX.2 [klein] 4B', description: 'Makes and edits images.', license: 'Apache-2.0',
  memory_bytes: 12 * 1024 ** 3, size_bytes: 5_207_178_964, edits: true, kind: 'image', installed: false, recommended: true, ...over,
})

function renderWith(status: ImageSetup) {
  vi.mocked(api.getMediaSetup).mockResolvedValue(status)
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <ImageSetupCard />
    </QueryClientProvider>,
  )
}

// The Tools page says when a model won't fit or will be slow here, and
// doesn't set up one that won't fit.
describe('ImageSetupCard', () => {
  it("won't set up a model this computer can't hold, and says why", async () => {
    renderWith({ supported: true, ready: false, program: false, release: 'r', memory_bytes: 4 * 1024 ** 3, accelerated: false, models: [model({ too_little_memory: true })] })
    expect(await screen.findByText(/Needs at least .* of memory; this computer has .*, so it can't run here\./)).toBeInTheDocument()
    expect(screen.getByText(/no GPU acceleration for this, so each picture takes a few minutes/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Set up/ })).not.toBeInTheDocument()
  })

  it('warns when memory is tight, and still sets it up', async () => {
    renderWith({ supported: true, ready: false, program: false, release: 'r', memory_bytes: 8 * 1024 ** 3, accelerated: true, models: [model({ tight_memory: true })] })
    expect(await screen.findByText(/less memory than it's comfortable with/)).toBeInTheDocument()
    expect(screen.queryByText(/no GPU acceleration/)).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Set up/ })).toBeInTheDocument()
  })

  it('says which build makes them, and offers the GPU build where one runs', async () => {
    vi.mocked(api.useMediaBuild).mockResolvedValue(null)
    renderWith({ supported: true, ready: true, program: true, release: 'r', active: 'flux2-klein-4b', accelerated: false, build: 'cpu', gpu_build: 'vulkan', models: [model({ installed: true })] })
    expect(await screen.findByText('Makes them on the CPU.')).toBeInTheDocument()
    expect(screen.queryByText(/no GPU acceleration/)).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Use the GPU build' }))
    await waitFor(() => expect(api.useMediaBuild).toHaveBeenCalledWith('images', 'gpu'))
  })

  it('offers the CPU build from the Vulkan build', async () => {
    renderWith({ supported: true, ready: true, program: true, release: 'r', active: 'flux2-klein-4b', accelerated: true, build: 'vulkan', gpu_build: 'vulkan', models: [model({ installed: true })] })
    expect(await screen.findByText('Makes them on the GPU (Vulkan).')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Use the CPU build' })).toBeInTheDocument()
  })
})
