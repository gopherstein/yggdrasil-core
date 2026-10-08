import { useEffect, useRef } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import { useRole } from '@/lib/role'
import { notifyDesktopBackgroundMode } from '@/lib/desktopBridge'
import { readScreenshotLaunch } from '@/lib/screenshotMode'

/** Saved schedules stop when the desktop app quits, so keep the daemon running. */
export function ScheduleBackgroundSync() {
  const queryClient = useQueryClient()
  const screenshot = readScreenshotLaunch()?.enabled ?? false
  const toldDesktop = useRef(false)
  // Keeping the service running is an Admin's setting (#203).
  const admin = useRole().atLeast('admin')
  const automations = useQuery({
    queryKey: ['automations'],
    queryFn: () => api.listAutomations(),
    enabled: !screenshot && admin,
    retry: false,
  })
  const settings = useQuery({
    queryKey: ['settings'],
    queryFn: () => api.getSettings(),
    enabled: !screenshot && admin,
    retry: false,
  })
  const scheduled = (automations.data?.length ?? 0) > 0

  useEffect(() => {
    if (!scheduled || toldDesktop.current) return
    toldDesktop.current = true
    void notifyDesktopBackgroundMode(true)
  }, [scheduled])

  useEffect(() => {
    if (!scheduled || settings.isLoading || settings.data?.keep_running_in_background) return
    let cancelled = false
    void api.updateSettings({ keep_running_in_background: true }).then(() => {
      if (!cancelled) {
        void queryClient.invalidateQueries({ queryKey: ['settings'] })
      }
    })
    return () => {
      cancelled = true
    }
  }, [queryClient, scheduled, settings.data?.keep_running_in_background, settings.isLoading])

  return null
}
