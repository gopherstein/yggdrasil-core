import { fireEvent, render, screen } from '@testing-library/react'
import { useState } from 'react'
import { describe, expect, it } from 'vitest'
import { TopicsEditor } from './TopicsEditor'
import { topicsDraft, topicsFrom } from './topicsDraft'

function Harness({ onDraft }: { onDraft: (d: ReturnType<typeof topicsDraft>) => void }) {
  const [draft, setDraft] = useState(topicsDraft())
  return (
    <TopicsEditor
      value={draft}
      onChange={(d) => {
        setDraft(d)
        onDraft(d)
      }}
    />
  )
}

describe('TopicsEditor (#345)', () => {
  it('turns the editor into topic controls, keeping strictness, or none', () => {
    expect(topicsFrom(topicsDraft())).toBeUndefined()
    expect(
      topicsFrom({ staysOn: ' Tires ', examples: 'Winter tires?\n\n Rotation? ', never: 'politics', reply: '', strict: 'enforce', sites: 'danastires.com\n', words: '' }),
    ).toEqual({ stays_on: 'Tires', examples: ['Winter tires?', 'Rotation?'], never_discuss: ['politics'], strictness: 'enforce', web_sites: ['danastires.com'] })
    expect(topicsDraft({ stays_on: 'Tires', examples: ['a', 'b'] }).examples).toBe('a\nb')
    expect(topicsDraft({ stays_on: 'Tires', strictness: 'enforce' }).strict).toBe('enforce')
    expect(topicsFrom({ ...topicsDraft(), staysOn: 'Tires' })).toEqual({ stays_on: 'Tires' })
  })

  it('shows the rest once there is something to stay on, with the default reply', () => {
    let last = topicsDraft()
    render(<Harness onDraft={(d) => (last = d)} />)
    expect(screen.queryByLabelText('Never discusses')).not.toBeInTheDocument()
    fireEvent.change(screen.getByLabelText(/^Stays on/), { target: { value: 'Tires and bookings' } })
    expect(screen.getByLabelText(/^Never discusses/)).toBeInTheDocument()
    expect(screen.getByPlaceholderText('I can only help with Tires and bookings. What can I help you with?')).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText(/^Example questions/), { target: { value: 'Winter tires?' } })
    expect(last.examples).toBe('Winter tires?')
    expect(screen.getByLabelText(/^Guide/)).toBeChecked()
    fireEvent.click(screen.getByLabelText(/^Enforce/))
    expect(last.strict).toBe('enforce')
    fireEvent.change(screen.getByLabelText(/^Web search only on/), { target: { value: 'danastires.com' } })
    expect(last.sites).toBe('danastires.com')
  })
})
