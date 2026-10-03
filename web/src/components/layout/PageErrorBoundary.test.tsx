import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { PageErrorBoundary } from './PageErrorBoundary'

let broken = true
function Page() {
  if (broken) throw new Error('models is undefined')
  return <p>Tools page</p>
}

afterEach(() => vi.restoreAllMocks())

describe('PageErrorBoundary', () => {
  it("keeps a page's error to that page, with a way to try again", () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    render(
      <>
        <nav>Sidebar</nav>
        <PageErrorBoundary>
          <Page />
        </PageErrorBoundary>
      </>,
    )
    expect(screen.getByRole('alert')).toHaveTextContent('This page ran into a problem')
    expect(screen.getByText('models is undefined')).toBeInTheDocument()
    expect(screen.getByText('Sidebar')).toBeInTheDocument()

    broken = false
    fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(screen.getByText('Tools page')).toBeInTheDocument()
  })
})
