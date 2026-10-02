import i18n from '@/i18n'
import { errorText } from '@/lib/api'

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

type Explained = { key: ErrorKey; actions: ErrorAction[] }

// Stable codes from the service (multilingual spec §10), which recognizes
// chat errors once, in core. The patterns below read the English text of a
// service from before codes, so an older computer's errors still read well.
const byCode: Record<string, Explained> = {
  SERVICE_UNREACHABLE: { key: 'notResponding', actions: ['retry'] },
  RUNTIME_NOT_INSTALLED: { key: 'engineMissing', actions: ['models'] },
  NO_MODEL_INSTALLED: { key: 'noModel', actions: ['models'] },
  NO_MODEL_ASSIGNED: { key: 'noModel', actions: ['models'] },
  MODEL_NOT_INSTALLED: { key: 'noModel', actions: ['models'] },
  CONTEXT_TOO_LONG: { key: 'tooLong', actions: ['new-chat', 'models'] },
  OUT_OF_MEMORY: { key: 'outOfMemory', actions: ['retry', 'models'] },
  COMPUTER_OFFLINE: { key: 'offline', actions: ['retry', 'computers'] },
  CONNECTION_LOST: { key: 'connection', actions: ['retry'] },
  RUNTIME_ERROR: { key: 'serverError', actions: ['retry', 'models'] },
}

const rules: { test: RegExp; error: Explained }[] = [
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

/** Turns an error from the service or the model into plain language, by its code when it has one. */
export function explainError(raw: string, code?: string): FriendlyError {
  const text = (raw || '').trim()
  const known = (code && byCode[code]) || rules.find((rule) => rule.test.test(text))?.error
  if (known) {
    const { key, actions } = known
    const body = i18n.t(`chat:errors.${key}.body`)
    return { title: i18n.t(`chat:errors.${key}.title`), body, actions, detail: text && text !== body ? text : undefined }
  }
  // A code the catalog explains reads in the App language, whatever the text's language.
  const coded = code ? errorText(code, text) : text
  if (coded !== text) {
    return { title: i18n.t('chat:errors.didNotWork'), body: coded, actions: ['retry'], detail: text || undefined }
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
