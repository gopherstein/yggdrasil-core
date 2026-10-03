import { useEffect } from 'react'
import { Outlet, useLocation } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { PageErrorBoundary } from '@/components/layout/PageErrorBoundary'
import { Sidebar } from '@/components/layout/Sidebar'

// Each page's name in the window title: common:nav.<key>.
const pageTitles: Record<string, string> = {
  '/chat': 'nav.chat',
  '/automations': 'nav.automations',
  '/models': 'nav.models',
  '/train': 'nav.train',
  '/knowledge': 'nav.knowledge',
  '/memory': 'nav.memory',
  '/nodes': 'nav.computers',
  '/performance': 'nav.performance',
  '/diagnostics': 'nav.diagnostics',
  '/profiles': 'nav.profiles',
  '/tools': 'nav.tools',
  '/api-access': 'nav.apiAccess',
  '/settings': 'nav.settings',
}

/** The window title for a path, such as "Models · Yggdrasil". */
function usePageTitle(pathname: string) {
  const { t } = useTranslation()
  const key = pageTitles[pathname]
  useEffect(() => {
    document.title = key ? `${t(key)} · Yggdrasil` : 'Yggdrasil'
  }, [key, t])
}

/**
 * App chrome: sidebar + main page region.
 * Overflow is owned here — pages scroll inside `.page-scroll`, not the OS window.
 */
export function AppLayout() {
  const { pathname } = useLocation()
  usePageTitle(pathname)
  return (
    <div className="app-shell">
      <Sidebar />
      <main className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
        <div className="page-scroll px-page-x py-5 sm:py-6">
          <PageErrorBoundary key={pathname}>
            <Outlet />
          </PageErrorBoundary>
        </div>
      </main>
    </div>
  )
}
