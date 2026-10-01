import { useLocation } from 'react-router-dom'
import { LoreButton } from '@/components/ui/LoreButton'
import { realmLore } from '@/lib/lore'
import { realmFor } from '@/lib/realms'
import { Rune } from './Rune'

export { Rune }

/**
 * The page's Norse name, set small above its title. It reads the current
 * route, so a page only has to place it. Clicking it opens a short lore entry.
 */
export function RealmKicker({ path, className = '' }: { path?: string; className?: string }) {
  const location = useLocation()
  const realm = realmFor(path ?? location.pathname)
  if (!realm) return null
  return (
    <div className={['flex', className].filter(Boolean).join(' ')}>
      <LoreButton lore={realmLore(realm)} className={['realm-kicker', realm.accent].join(' ')}>
        <Rune id={realm.rune} className="h-4 w-2.5" />
        <span>{realm.norse}</span>
      </LoreButton>
    </div>
  )
}
