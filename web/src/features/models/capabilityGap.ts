import i18n from '@/i18n'
import { modelUsesTools } from './modelPresentation'
import type { Model } from '@/types/api'

export type ModelSuggestion = {
  id: string
  name: string
  installed: boolean
}

export type CapabilityNote = {
  text: string
  suggestions: ModelSuggestion[]
  action?: 'enable-internet'
}

export type CapabilityGap = {
  notes: CapabilityNote[]
}

function seesImages(model: Pick<Model, 'capabilities' | 'tags'>): boolean {
  return Boolean(model.capabilities?.vision || model.tags?.includes('vision'))
}

function needsWeb(text: string): boolean {
  const value = text.toLowerCase()
  if (/\b(search|browse|look up|google)\b.{0,40}\b(web|online|internet)\b/.test(value)) return true
  if (/\b(web|online|internet)\b.{0,40}\b(search|browse|look up)\b/.test(value)) return true
  if (/\b(current|today'?s|right now|latest|live)\b.{0,40}\b(weather|news|score|stock|price)\b/.test(value)) {
    return true
  }
  if (/\b(weather|forecast)\b.{0,40}\b(today|now|right now|current|tomorrow)\b/.test(value)) return true
  return false
}

function needsLocalTools(text: string): boolean {
  if (/\bgit\s+(status|diff|add|commit|log)\b/i.test(text)) return true
  if (/\b(read|open|edit|update|write|change)\b.{0,48}\b(file|readme|directory|folder|repo)\b/i.test(text)) {
    return true
  }
  if (/\b(run|execute)\b.{0,32}\b(command|terminal|shell|script)\b/i.test(text)) return true
  if (/\b(in|from)\s+my\s+(workspace|repo|repository|project)\b/i.test(text)) return true
  return false
}

function needsVision(text: string): boolean {
  if (/\b(this|the|attached|my|uploaded)\s+(image|screenshot|photo|picture|diagram)\b/i.test(text)) {
    return true
  }
  return /\b(look at|describe|what is in)\b.{0,24}\b(image|screenshot|photo|picture)\b/i.test(text)
}

function suggest(
  catalog: Model[],
  currentId: string,
  match: (model: Model) => boolean,
): ModelSuggestion[] {
  return catalog
    .filter((model) => model.id !== currentId && match(model))
    .sort((a, b) => {
      if (Boolean(a.installed) !== Boolean(b.installed)) return a.installed ? -1 : 1
      return (a.memory_needed_bytes ?? Number.MAX_SAFE_INTEGER) - (b.memory_needed_bytes ?? Number.MAX_SAFE_INTEGER)
    })
    .slice(0, 3)
    .map((model) => ({
      id: model.id,
      name: model.display_name || model.id,
      installed: Boolean(model.installed),
    }))
}

/** Suggest other models when this request needs something the current model cannot do. */
export function capabilityGap(
  message: string,
  model: Pick<Model, 'id' | 'display_name' | 'capabilities' | 'tags'> | null,
  catalog: Model[],
  options?: { terminalAllowed?: boolean; internetAllowed?: boolean },
): CapabilityGap | null {
  const text = message.trim()
  if (!text || !model) return null
  const name = model.display_name || model.id
  const internetAllowed = options?.internetAllowed !== false
  const notes: CapabilityNote[] = []

  if (needsWeb(text) && !(modelUsesTools(model) && internetAllowed)) {
    if (!internetAllowed) {
      notes.push({
        text: i18n.t('models:capability.noWeb'),
        suggestions: modelUsesTools(model) ? [] : suggest(catalog, model.id, modelUsesTools),
        action: 'enable-internet',
      })
    } else {
      notes.push({
        text: i18n.t('models:capability.noTools', { model: name }),
        suggestions: suggest(catalog, model.id, modelUsesTools),
      })
    }
  }
  if (needsLocalTools(text) && !modelUsesTools(model)) {
    notes.push({
      text: i18n.t('models:capability.noLocalTools', { model: name }),
      suggestions: suggest(catalog, model.id, modelUsesTools),
    })
  }
  // With a vision model installed, Toskar answers a picture with it, so the
  // note is only for when none is.
  if (needsVision(text) && !seesImages(model) && !catalog.some((m) => m.installed && seesImages(m))) {
    notes.push({
      text: i18n.t('models:capability.noVision', { model: name }),
      suggestions: suggest(catalog, model.id, seesImages),
    })
  }
  if (notes.length === 0) return null
  return { notes }
}
