import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { applyLanguage } from '@/i18n'
import { api } from '@/lib/api'
import type { ImageSetup, SetupOffer } from '@/types/api'
import { SetupOfferCard } from './SetupOffer'

vi.mock('@/lib/api', () => ({
  api: { getMediaSetup: vi.fn(), startMediaSetup: vi.fn(), cancelMediaSetup: vi.fn() },
}))

const offer: SetupOffer = {
  ability: 'image_generation',
  label: 'Generate images',
  option: 'flux2-klein-4b',
  name: 'FLUX.2 [klein] 4B',
  size_bytes: 5_207_178_964,
  node_name: 'Studio',
  request: 'Make me an image of a Viking tree',
}

function status(over: Partial<ImageSetup>): ImageSetup {
  return { supported: true, ready: false, program: false, release: 'r', models: [], ...over }
}

function renderIt(onContinue: (text: string) => void, o: SetupOffer = offer) {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <SetupOfferCard offer={o} onContinue={onContinue} />
    </QueryClientProvider>,
  )
}

describe('SetupOfferCard', () => {
  beforeEach(async () => {
    await applyLanguage('en')
    vi.mocked(api.getMediaSetup).mockReset()
    vi.mocked(api.startMediaSetup).mockReset()
  })

  it('says what it would install, then finishes the request once it is ready', async () => {
    vi.mocked(api.getMediaSetup).mockResolvedValue(status({}))
    const onContinue = vi.fn()
    renderIt(onContinue)
    expect(await screen.findByText('FLUX.2 [klein] 4B · 5.2 GB download · on Studio (this computer)')).toBeInTheDocument()

    vi.mocked(api.startMediaSetup).mockResolvedValue(
      status({ ready: true, program: true, active: 'flux2-klein-4b', job: { model_id: 'flux2-klein-4b', stage: 'model', done_bytes: 1, total_bytes: 1, running: false } }),
    )
    fireEvent.click(screen.getByRole('button', { name: 'Set up' }))
    await waitFor(() => expect(onContinue).toHaveBeenCalledWith('Make me an image of a Viking tree'))
    expect(api.startMediaSetup).toHaveBeenCalledWith('images', 'flux2-klein-4b')
    expect(await screen.findByText('Set up. Finishing your request…')).toBeInTheDocument()
    expect(onContinue).toHaveBeenCalledTimes(1)
  })

  it('offers to continue when it was set up elsewhere', async () => {
    vi.mocked(api.getMediaSetup).mockResolvedValue(status({ ready: true, program: true, active: 'flux2-klein-4b' }))
    const onContinue = vi.fn()
    renderIt(onContinue)
    fireEvent.click(await screen.findByRole('button', { name: 'Continue' }))
    expect(onContinue).toHaveBeenCalledWith('Make me an image of a Viking tree')
  })

  it('shows the download while it runs', async () => {
    vi.mocked(api.getMediaSetup).mockResolvedValue(
      status({ job: { model_id: 'flux2-klein-4b', stage: 'model', done_bytes: 2_600_000_000, total_bytes: 5_200_000_000, running: true } }),
    )
    renderIt(vi.fn())
    expect(await screen.findByText('Downloading… 2.6 GB of 5.2 GB')).toBeInTheDocument()
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '50')
  })

  it('sets up video generation for a video request', async () => {
    vi.mocked(api.getMediaSetup).mockResolvedValue(status({}))
    vi.mocked(api.startMediaSetup).mockResolvedValue(status({ ready: true }))
    const onContinue = vi.fn()
    renderIt(onContinue, { ...offer, ability: 'video_generation', option: 'wan2.2-ti2v-5b', name: 'Wan 2.2 TI2V 5B', size_bytes: 8_497_662_272, request: 'Make a video of waves' })
    fireEvent.click(await screen.findByRole('button', { name: 'Set up' }))
    await waitFor(() => expect(onContinue).toHaveBeenCalledWith('Make a video of waves'))
    expect(api.getMediaSetup).toHaveBeenCalledWith('video')
    expect(api.startMediaSetup).toHaveBeenCalledWith('video', 'wan2.2-ti2v-5b')
  })
})
