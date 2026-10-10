import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { Conversation } from '@/types/api'
import { ChatHistoryDrawer } from './ChatHistoryDrawer'

const now = new Date().toISOString()
const chats: Conversation[] = ['Alpha', 'Bravo', 'Charlie', 'Delta'].map((title, i) => ({
  id: `c${i + 1}`,
  title,
  created_at: now,
  updated_at: now,
}))

function drawer(onDeleteMany = vi.fn(), onSelect = vi.fn()) {
  render(
    <ChatHistoryDrawer
      open
      mode="pinned"
      conversations={chats}
      pinnedIds={[]}
      selectedId="c1"
      onClose={vi.fn()}
      onSelect={onSelect}
      onNewChat={vi.fn()}
      onRename={vi.fn()}
      onTogglePin={vi.fn()}
      onDelete={vi.fn()}
      onDeleteMany={onDeleteMany}
      onToggleDrawerPinned={vi.fn()}
      drawerPinned
      canPinDrawer={false}
      renamingId={null}
      renameValue=""
      onRenameValueChange={vi.fn()}
      onCommitRename={vi.fn()}
      onCancelRename={vi.fn()}
    />,
  )
  return { onDeleteMany, onSelect }
}

const titles = (calls: Conversation[][][]) => calls[0][0].map((c) => c.title)

describe('ChatHistoryDrawer select mode (#452)', () => {
  it('chooses chats, a Shift-click range, and deletes them together', () => {
    const { onDeleteMany, onSelect } = drawer()
    fireEvent.click(screen.getByRole('button', { name: 'Select' }))
    fireEvent.click(screen.getByRole('checkbox', { name: 'Bravo' }))
    fireEvent.click(screen.getByRole('checkbox', { name: 'Delta' }), { shiftKey: true })
    expect(screen.getByText('3 selected')).toBeInTheDocument()
    expect(onSelect).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: 'Delete 3' }))
    expect(titles(onDeleteMany.mock.calls)).toEqual(['Bravo', 'Charlie', 'Delta'])
  })

  it('starts choosing with Cmd-click or Shift-click from the open chat, and Escape leaves', () => {
    const { onSelect } = drawer()
    fireEvent.click(screen.getByTitle('Charlie'), { shiftKey: true })
    expect(screen.getByText('3 selected')).toBeInTheDocument()
    fireEvent.keyDown(screen.getByText('3 selected'), { key: 'Escape' })
    expect(screen.queryByText(/selected/)).not.toBeInTheDocument()
    fireEvent.click(screen.getByTitle('Delta'), { metaKey: true })
    expect(screen.getByText('1 selected')).toBeInTheDocument()
    expect(onSelect).not.toHaveBeenCalled()
  })

  it('selects all of the chats a search shows', () => {
    const { onDeleteMany } = drawer()
    fireEvent.change(screen.getByRole('searchbox'), { target: { value: 'a' } })
    fireEvent.click(screen.getByRole('button', { name: 'Select' }))
    fireEvent.change(screen.getByRole('searchbox'), { target: { value: 'ar' } })
    fireEvent.click(screen.getByRole('button', { name: 'Select all' }))
    fireEvent.click(screen.getByRole('button', { name: 'Delete 1' }))
    expect(titles(onDeleteMany.mock.calls)).toEqual(['Charlie'])
  })
})
