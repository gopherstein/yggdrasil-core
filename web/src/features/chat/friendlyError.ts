export type ErrorAction = 'retry' | 'models' | 'new-chat' | 'computers'

export interface FriendlyError {
  title: string
  body: string
  actions: ErrorAction[]
  /** The raw text, shown under Details when it adds something. */
  detail?: string
}

const rules: { test: RegExp; error: Omit<FriendlyError, 'detail'> }[] = [
  {
    test: /could not reach the local yggdrasil service|failed to fetch|networkerror/i,
    error: {
      title: "Yggdrasil isn't responding",
      body: 'The local service stopped answering. If this is the desktop app, quit and reopen it, then try again.',
      actions: ['retry'],
    },
  },
  {
    test: /llama-server (is )?not installed/i,
    error: {
      title: "The AI engine isn't installed yet",
      body: 'Install it from Models, then send your message again.',
      actions: ['models'],
    },
  },
  {
    test: /no model assigned|needs model assignments|install a model|no installed model|model .* not installed/i,
    error: { title: 'No model is ready', body: 'Install or choose a model, then try again.', actions: ['models'] },
  },
  {
    test: /context (length|size|window)|too many tokens|exceeds the (available )?context|n_ctx/i,
    error: {
      title: 'This conversation is too long for the model',
      body: 'Start a new chat, or choose a model that can hold more text.',
      actions: ['new-chat', 'models'],
    },
  },
  {
    test: /out of memory|\boom\b|failed to allocate|insufficient memory|not enough memory/i,
    error: {
      title: 'Not enough memory for this model',
      body: 'Close other apps or choose a smaller model, then try again.',
      actions: ['retry', 'models'],
    },
  },
  {
    test: /is offline|offline\.|unreachable|no route to host/i,
    error: {
      title: "A computer you're using is offline",
      body: 'Turn it on and open Yggdrasil there, or run this chat on this computer.',
      actions: ['retry', 'computers'],
    },
  },
  {
    test: /econnreset|econnrefused|connection (reset|refused)|broken pipe|unexpected eof|\beof\b|deadline exceeded|timed? ?out|i\/o timeout/i,
    error: {
      title: "Couldn't reach the model",
      body: 'The model stopped responding. This is usually temporary.',
      actions: ['retry'],
    },
  },
  {
    test: /llama-server error 5\d\d|http 5\d\d|internal server error/i,
    error: { title: 'The model ran into a problem', body: 'Try again. If it keeps happening, try another model.', actions: ['retry', 'models'] },
  },
]

/** A plain sentence is already fit to show; machine text is not. */
function readable(raw: string): boolean {
  return (
    raw.length < 220 &&
    /^[A-Z]/.test(raw) &&
    /[.!?]$/.test(raw) &&
    !/[{}<>]|\b[a-z_]+\.[a-z_]+\(|:\s*\d{3}\b/.test(raw)
  )
}

/** Turns an error from the service or the model into plain language. */
export function explainError(raw: string): FriendlyError {
  const text = (raw || '').trim()
  for (const rule of rules) {
    if (rule.test.test(text)) {
      return { ...rule.error, detail: text && text !== rule.error.body ? text : undefined }
    }
  }
  if (text && readable(text)) {
    return { title: 'That didn’t work', body: text, actions: ['retry'] }
  }
  return {
    title: 'Something went wrong',
    body: 'Yggdrasil could not finish that reply. Try again in a moment.',
    actions: ['retry'],
    detail: text || undefined,
  }
}
