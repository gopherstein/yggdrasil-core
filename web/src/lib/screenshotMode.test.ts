import { describe, expect, it } from 'vitest'
import { screenshotPath } from './screenshotMode'

describe('screenshotPath', () => {
  it('maps App Store screen ids onto existing routes', () => {
    expect(screenshotPath('chat')).toBe('/chat')
    expect(screenshotPath('models')).toBe('/models')
    expect(screenshotPath('computers')).toBe('/nodes')
    expect(screenshotPath('performance')).toBe('/performance')
    expect(screenshotPath('diagnostics')).toBe('/diagnostics')
    expect(screenshotPath('api-manager')).toBe('/api-access')
    expect(screenshotPath('automations')).toBe('/automations')
    expect(screenshotPath('automations-create')).toBe('/automations')
    expect(screenshotPath('unknown')).toBe('/chat')
  })
})
