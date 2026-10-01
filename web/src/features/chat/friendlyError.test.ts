import { describe, expect, it } from 'vitest'
import { explainError } from './friendlyError'

describe('explainError', () => {
  it.each([
    ['llama-server not installed: llama-server is not installed. Install from the Runtimes page', "The AI engine isn't installed yet", ['models']],
    ['llama-server error 400: {"error":{"message":"the request exceeds the available context size"}}', 'This conversation is too long for the model', ['new-chat', 'models']],
    ['read tcp 127.0.0.1:51234->127.0.0.1:8081: read: connection reset by peer', "Couldn't reach the model", ['retry']],
    ['ggml_metal: failed to allocate buffer, out of memory', 'Not enough memory for this model', ['retry', 'models']],
    ['Studio is offline. Turn that computer on and open Yggdrasil, or change the role to Automatic placement.', "A computer you're using is offline", ['retry', 'computers']],
    ['llama-server error 500: Internal Server Error', 'The model ran into a problem', ['retry', 'models']],
    ['profile "x" needs model assignments (or pass a model id); no installed model', 'No model is ready', ['models']],
  ])('explains %s', (raw, title, actions) => {
    const e = explainError(raw)
    expect(e.title).toBe(title)
    expect(e.actions).toEqual(actions)
    expect(e.detail).toBe(raw)
  })

  it('passes a plain sentence through', () => {
    const e = explainError('Tread Right is not deployed yet. Finish training and deploy it on the Train page.')
    expect(e.title).toBe('That didn’t work')
    expect(e.body).toMatch(/^Tread Right is not deployed yet/)
    expect(e.detail).toBeUndefined()
  })

  it('hides machine text behind Details', () => {
    const e = explainError('panic: runtime error: index out of range [3] with length 3')
    expect(e.title).toBe('Something went wrong')
    expect(e.body).not.toMatch(/panic/)
    expect(e.detail).toMatch(/^panic/)
  })
})
