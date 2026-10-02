import i18n from '@/i18n'

export type ErrorAction = 'retry' | 'models' | 'new-chat' | 'computers'

export interface FriendlyError {
  title: string
  body: string
  actions: ErrorAction[]
  /** The raw text, shown under Details when it adds something. */
  detail?: string
}

// Each rule's title and body are chat:errors.<key>.title and .body in the catalog.
type ErrorKey = 'notResponding' | 'engineMissing' | 'noModel' | 'tooLong' | 'outOfMemory' | 'offline' | 'connection' | 'serverError'

const rules: { test: RegExp; error: { key: ErrorKey; actions: ErrorAction[] } }[] = [
  {
    test: /could not reach the local yggdrasil service|failed to fetch|networkerror/i,
    error: {
      key: 'notResponding',
      actions: ['retry'],
    },
  },
  {
    test: /llama-server (is )?not installed/i,
    error: {
      key: 'engineMissing',
      actions: ['models'],
    },
  },
  {
    test: /no model assigned|needs model assignments|install a model|no installed model|model .* not installed/i,
    error: { key: 'noModel', actions: ['models'] },
  },
  {
    test: /context (length|size|window)|too many tokens|exceeds the (available )?context|n_ctx/i,
    error: {
      key: 'tooLong',
      actions: ['new-chat', 'models'],
    },
  },
  {
    test: /out of memory|\boom\b|failed to allocate|insufficient memory|not enough memory/i,
    error: {
      key: 'outOfMemory',
      actions: ['retry', 'models'],
    },
  },
  {
    test: /is offline|offline\.|unreachable|no route to host/i,
    error: {
      key: 'offline',
      actions: ['retry', 'computers'],
    },
  },
  {
    test: /econnreset|econnrefused|connection (reset|refused)|broken pipe|unexpected eof|\beof\b|deadline exceeded|timed? ?out|i\/o timeout/i,
    error: {
      key: 'connection',
      actions: ['retry'],
    },
  },
  {
    test: /llama-server error 5\d\d|http 5\d\d|internal server error/i,
    error: { key: 'serverError', actions: ['retry', 'models'] },
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
      const { key, actions } = rule.error
      const body = i18n.t(`chat:errors.${key}.body`)
      return { title: i18n.t(`chat:errors.${key}.title`), body, actions, detail: text && text !== body ? text : undefined }
    }
  }
  if (text && readable(text)) {
    return { title: i18n.t('chat:errors.didNotWork'), body: text, actions: ['retry'] }
  }
  return {
    title: i18n.t('chat:errors.generic.title'),
    body: i18n.t('chat:errors.generic.body'),
    actions: ['retry'],
    detail: text || undefined,
  }
}
