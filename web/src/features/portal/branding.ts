import type { CSSProperties } from 'react'
import type { PortalBranding } from '@/types/api'

// A portal's branding (#205) comes from its Admin, as JSON the service
// doesn't look inside. Each field is checked here before it's used: only
// hex colours, an http(s) or image data address for the logo, and text,
// which React shows as text.

const hexRe = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i

/** A hex colour as the "r g b" channels the theme reads, or null. */
export function channels(hex: string | undefined): string | null {
  if (!hex || !hexRe.test(hex.trim())) return null
  let h = hex.trim().slice(1)
  if (h.length === 3) h = h.replace(/./g, (c) => c + c)
  const n = Number.parseInt(h, 16)
  return `${(n >> 16) & 255} ${(n >> 8) & 255} ${n & 255}`
}

/** Text that reads on a colour: white on dark ones, near-black on light. */
export function readableOn(rgb: string): string {
  const [r, g, b] = rgb.split(' ').map((v) => Number(v) / 255)
  const lin = (c: number) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4)
  const luminance = 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b)
  return luminance > 0.4 ? '11 15 20' : '255 255 255'
}

/** The logo's address when it's one a page may show, else undefined. */
export function safeLogo(url: string | undefined): string | undefined {
  if (!url) return undefined
  const u = url.trim()
  if (/^https?:\/\/[^\s"'<>]+$/i.test(u)) return u
  if (/^data:image\/(png|jpeg|gif|webp|svg\+xml);base64,[a-z0-9+/=]+$/i.test(u)) return u
  return undefined
}

/** The suggested first messages: up to six short ones. */
export function prompts(branding: PortalBranding): string[] {
  return (Array.isArray(branding.prompts) ? branding.prompts : [])
    .filter((p): p is string => typeof p === 'string' && p.trim() !== '')
    .map((p) => p.trim().slice(0, 200))
    .slice(0, 6)
}

/** The theme a portal asks for, with the visitor's own for system. */
export function theme(branding: PortalBranding, prefersDark: boolean): 'dark' | 'light' {
  if (branding.theme === 'light' || branding.theme === 'dark') return branding.theme
  return prefersDark ? 'dark' : 'light'
}

/** The page's colours: the accent for buttons and focus, and the background. */
export function themeVars(branding: PortalBranding): CSSProperties {
  const vars: Record<string, string> = {}
  const accent = channels(branding.accent)
  if (accent) {
    vars['--rgb-primary'] = accent
    vars['--rgb-primary-hover'] = accent
    vars['--rgb-primary-fg'] = readableOn(accent)
  }
  const background = channels(branding.background)
  if (background) vars['--rgb-canvas'] = background
  return vars as CSSProperties
}

/** Short text from the branding, or fallback. */
export function text(value: unknown, fallback = '', max = 500): string {
  return typeof value === 'string' && value.trim() ? value.trim().slice(0, max) : fallback
}
