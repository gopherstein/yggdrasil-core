import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { Message } from '@/types/api'
import { AnswerActions, EditAction, VersionSwitch } from './MessageActions'

const answer = (versions?: Message['versions']): Message => ({
  id: 'b',
  conversation_id: 'c',
  role: 'assistant',
  content: 'An answer',
  created_at: '2026-10-08T00:00:00Z',
  versions,
})

describe('MessageActions (#447)', () => {
  it('switches between versions, only when there is more than one', () => {
    const onShow = vi.fn()
    const { rerender, container } = render(<VersionSwitch message={answer()} onShow={onShow} />)
    expect(container).toBeEmptyDOMElement()
    rerender(<VersionSwitch message={answer({ index: 2, count: 3, ids: ['a', 'b', 'c'] })} onShow={onShow} />)
    expect(screen.getByText('2 / 3')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Previous version' }))
    fireEvent.click(screen.getByRole('button', { name: 'Next version' }))
    expect(onShow.mock.calls).toEqual([['a'], ['c']])
    rerender(<VersionSwitch message={answer({ index: 1, count: 3, ids: ['a', 'b', 'c'] })} onShow={onShow} />)
    expect(screen.getByRole('button', { name: 'Previous version' })).toBeDisabled()
  })

  it('tries again, or again with another model', () => {
    const onRetry = vi.fn()
    render(
      <AnswerActions
        models={[
          { id: 'auto', name: 'Auto' },
          { id: 'gemma', name: 'Gemma 3 4B' },
        ]}
        onRetry={onRetry}
      />,
    )
    fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    fireEvent.change(screen.getByLabelText('Try again with another model'), { target: { value: 'gemma' } })
    expect(onRetry.mock.calls).toEqual([[], ['gemma']])
  })

  it('edits a sent message', () => {
    const onEdit = vi.fn()
    render(<EditAction onEdit={onEdit} />)
    fireEvent.click(screen.getByRole('button', { name: 'Edit' }))
    expect(onEdit).toHaveBeenCalled()
  })
})
