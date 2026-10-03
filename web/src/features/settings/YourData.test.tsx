import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { YourData } from './YourData'

describe('YourData', () => {
  it('says where data lives and what is encrypted', () => {
    render(<YourData />)
    expect(screen.getByRole('heading', { name: 'Your data stays private' })).toBeInTheDocument()
    expect(screen.getByText(/stored on this computer/)).toBeInTheDocument()
    expect(screen.getByText(/FileVault/)).toBeInTheDocument()
    expect(screen.getByText(/between your own computers/)).toBeInTheDocument()
  })
})
