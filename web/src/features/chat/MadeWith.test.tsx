import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { applyLanguage } from '@/i18n'
import type { FileRef } from '@/types/api'
import { FileChip } from './FileChips'

vi.mock('@/lib/api', () => ({
  artifactObjectUrl: vi.fn(() => Promise.resolve('blob:x')),
  downloadArtifact: vi.fn(),
}))

const picture: FileRef = {
  id: 'a1',
  name: 'dog.png',
  mime_type: 'image/png',
  kind: 'image',
  size_bytes: 10,
  producer: 'assistant',
  details: { model: 'Z-Image-Turbo', seed: 4211, width: 1024, height: 576, prompt: 'A dog on a beach' },
}

describe('made pictures and clips', () => {
  beforeEach(async () => {
    await applyLanguage('en')
  })

  it('say how they were made, with the prompt on hover (#553)', async () => {
    render(<FileChip file={picture} />)
    const line = await screen.findByText('Z-Image-Turbo · 1024×576 · seed 4211')
    expect(line).toHaveAttribute('title', 'A dog on a beach')
  })

  it('say nothing when there are no details', () => {
    render(<FileChip file={{ ...picture, details: undefined }} />)
    expect(screen.queryByText(/seed/)).toBeNull()
  })

  it('say which computer made it, when another one did', async () => {
    render(<FileChip file={{ ...picture, details: { ...picture.details, computer: 'Workstation' } }} />)
    expect(await screen.findByText('Z-Image-Turbo · 1024×576 · seed 4211 · on Workstation')).toBeInTheDocument()
  })
})
