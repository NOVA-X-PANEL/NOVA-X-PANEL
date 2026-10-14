// @ts-nocheck — vendored from Heimdall-Panel (upstream pg-ui); keep byte-compatible with upstream.
import { useQuery } from '@tanstack/react-query';

type GoalType = 'donation' | 'github_stars' | (string & {})

interface Goal {
  id: number
  name: string
  detail: string
  price: number
  paid_amount: number
  status: 'pending' | 'completed' | 'cancelled'
  type: GoalType
  repo_owner?: string
  repo_name?: string
  created_at: string
  updated_at: string
}

interface GoalsResponse {
  next_pending: Goal[]
  last_completed: Goal[]
  last_cancelled: Goal[]
  pending_count: number
  completed_count: number
  cancelled_count: number
}

export function useAllGoals() {
  return useQuery({
    queryKey: ['all-goals'],
    queryFn: async () => {
      return null;
    },
    enabled: false,
  });
}
