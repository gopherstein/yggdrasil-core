import i18n from '@/i18n'
import type { Accelerator, HardwareInventory, Model, Node } from '@/types/api'
import { purposeChipIds } from '@/features/models/modelPresentation'

function isMeaningful(s?: string | null): s is string {
  if (!s) return false
  const t = s.trim()
  if (!t || t === '.' || t === '/' || t === '-' || t === '—') return false
  if (t === '/' || t.includes('/')) {
    const parts = t.split('/').map((p) => p.trim())
    if (parts.every((p) => !p || p === '.')) return false
  }
  return true
}

function gbLabel(bytes?: number): string | null {
  if (bytes == null || bytes <= 0) return null
  const gb = bytes / 1024 ** 3
  return i18n.t('computers:hardware.gigabytes', { value: gb >= 10 ? Math.round(gb) : gb.toFixed(1) })
}

function accelLine(accel?: Accelerator): string | null {
  if (!accel) return null
  const name = [accel.model, accel.vendor].find(isMeaningful)
  if (!name) return null
  const dedicated = gbLabel(accel.dedicated_vram_bytes)
  if (dedicated) return `${name} · ${i18n.t('computers:hardware.vram', { size: dedicated })}`
  const unified = gbLabel(accel.unified_memory_bytes)
  if (unified) return `${name} · ${i18n.t('computers:hardware.unified', { size: unified })}`
  return name
}

export type NodeHardwareView = {
  primary: string | null
  secondary: string | null
  memoryBytes: number
  unavailable: boolean
}

export function describeNodeHardware(hw?: HardwareInventory | null): NodeHardwareView {
  if (!hw) {
    return { primary: null, secondary: null, memoryBytes: 0, unavailable: true }
  }

  const cpu = isMeaningful(hw.cpu?.model) ? hw.cpu.model.trim() : null
  const accel = hw.accelerators?.[0]
  const accelText = accelLine(accel)
  const ram = gbLabel(hw.memory?.total_bytes)
  const memBytes = hw.memory?.total_bytes ?? 0
  const unified = accel?.unified_memory_bytes
    ? gbLabel(accel.unified_memory_bytes)
    : null

  // Apple-style: chip as primary, unified memory as secondary
  if (cpu && /apple|m[0-9]/i.test(cpu) && (unified || ram)) {
    return {
      primary: cpu,
      secondary: unified
        ? i18n.t('computers:hardware.unified', { size: unified })
        : i18n.t('computers:hardware.memory', { size: ram }),
      memoryBytes: unified
        ? accel!.unified_memory_bytes!
        : memBytes,
      unavailable: false,
    }
  }

  if (cpu && accelText) {
    return {
      primary: cpu,
      secondary: accelText,
      memoryBytes: memBytes || accel?.dedicated_vram_bytes || 0,
      unavailable: false,
    }
  }

  if (accelText) {
    return {
      primary: accelText,
      secondary: ram ? i18n.t('computers:hardware.ram', { size: ram }) : null,
      memoryBytes: memBytes || accel?.dedicated_vram_bytes || 0,
      unavailable: false,
    }
  }

  if (cpu) {
    return {
      primary: cpu,
      secondary: ram ? i18n.t('computers:hardware.ram', { size: ram }) : null,
      memoryBytes: memBytes,
      unavailable: false,
    }
  }

  if (ram) {
    return {
      primary: i18n.t('computers:hardware.memory', { size: ram }),
      secondary: null,
      memoryBytes: memBytes,
      unavailable: false,
    }
  }

  return { primary: null, secondary: null, memoryBytes: 0, unavailable: true }
}

export type MembershipKind = 'local' | 'paired' | 'nearby' | 'offline'

export function membershipKind(node: Node): MembershipKind {
  if (node.is_local) return 'local'
  if (!node.paired) return 'nearby'
  if (node.status === 'offline') return 'offline'
  return 'paired'
}

export function membershipLabel(kind: MembershipKind): string {
  switch (kind) {
    case 'local':
      return i18n.t('computers:membership.local')
    case 'paired':
      return i18n.t('computers:membership.paired')
    case 'nearby':
      return i18n.t('computers:membership.nearby')
    case 'offline':
      return i18n.t('computers:membership.offline')
  }
}

export type OnlineState = 'online' | 'offline' | 'checking'

export function onlineState(node: Node): OnlineState {
  if (node.is_local || node.status === 'online') return 'online'
  if (node.status === 'offline') return 'offline'
  return 'checking'
}

export function onlineLabel(node: Node): string {
  return i18n.t(`computers:online.${onlineState(node)}`)
}

/** What a computer is good for; each is computers:abilities.<ability> in the catalog. */
export type Ability = 'general' | 'coding' | 'reasoning' | 'vision' | 'tools' | 'fast' | 'large'

const abilityLabel = (ability: Ability) => i18n.t(`computers:abilities.${ability}`)

function abilities(models: Model[], hw?: HardwareInventory | null): Set<Ability> {
  const set = new Set<Ability>()
  for (const m of models) {
    for (const chip of purposeChipIds(m)) set.add(chip)
    if (m.tags?.includes('large')) set.add('large')
  }

  const mem =
    hw?.memory?.total_bytes ??
    hw?.accelerators?.[0]?.unified_memory_bytes ??
    hw?.accelerators?.[0]?.dedicated_vram_bytes ??
    0
  const vram =
    hw?.accelerators?.[0]?.dedicated_vram_bytes ??
    hw?.accelerators?.[0]?.unified_memory_bytes ??
    0

  if (set.size === 0) {
    if (mem >= 24 * 1024 ** 3 || vram >= 16 * 1024 ** 3) set.add('large')
    if (vram >= 8 * 1024 ** 3) set.add('vision')
    set.add('general')
    set.add('coding')
  } else if (mem >= 48 * 1024 ** 3 || vram >= 20 * 1024 ** 3) {
    set.add('large')
  }
  return set
}

function topAbilities(models: Model[], hw?: HardwareInventory | null): Ability[] {
  const set = abilities(models, hw)
  const order: Ability[] = ['general', 'coding', 'reasoning', 'vision', 'tools', 'fast', 'large']
  return order.filter((a) => set.has(a)).slice(0, 4)
}

/** What this computer is good for, from installed models + hardware hints. */
export function availableForLabels(models: Model[], hw?: HardwareInventory | null): string[] {
  return topAbilities(models, hw).map(abilityLabel)
}

export function teamBestAt(fleet: Node[], models: Model[]): string[] {
  const set = new Set<Ability>()
  for (const n of fleet) {
    const onNode = models.filter((m) =>
      (m.installed_on ?? []).some((p) => p.node_id === n.id),
    )
    for (const ability of topAbilities(onNode, n.hardware)) set.add(ability)
  }
  const order: Ability[] = ['coding', 'vision', 'reasoning', 'large', 'general', 'tools', 'fast']
  return order.filter((a) => set.has(a)).slice(0, 3).map(abilityLabel)
}

export function combinedMemoryBytes(fleet: Node[]): number {
  let total = 0
  for (const n of fleet) {
    const view = describeNodeHardware(n.hardware)
    total += view.memoryBytes
  }
  return total
}
