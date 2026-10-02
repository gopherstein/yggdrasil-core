import type { OrchestrationPolicy } from '@/types/api'

/** Which controls go in which section of the profile editor (spec §20). */
export const ORCHESTRATION_KEYS: (keyof OrchestrationPolicy)[] = [
  'effort',
  'planning',
  'parallel',
  'verification',
  'max_workers',
  'max_tool_calls',
]
export const MEMORY_KEYS: (keyof OrchestrationPolicy)[] = ['memory', 'context_share']
export const EXECUTION_KEYS: (keyof OrchestrationPolicy)[] = ['fallback', 'retries', 'timeout_seconds']
