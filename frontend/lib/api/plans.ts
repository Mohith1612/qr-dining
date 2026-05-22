import { api } from "@/lib/api/client"

export type PlanTier = "free" | "standard" | "premium"

export type Plan = {
  id: number
  name: string
  tier: PlanTier
  price_monthly: number
  features_json: {
    max_branches: number
    max_tables: number
    analytics: boolean
    multi_branch: boolean
  }
  created_at: string
}

export type Subscription = {
  id: number
  restaurant_id: number
  plan_id: number
  status: "trial" | "active" | "expired" | "cancelled"
  trial_ends_at: string | null
  current_period_start: string
  current_period_end: string | null
  plan_name: string
  plan_tier: PlanTier
  plan_price_monthly: number
  plan_features_json: Plan["features_json"]
}

export const plansApi = {
  listPlans: () =>
    api.get<{ plans: Plan[] }>("/plans"),

  getSubscription: (restaurantId: number, staffToken: string) =>
    api.get<{ subscription: Subscription; features: Plan["features_json"] }>(
      `/restaurants/${restaurantId}/subscription`,
      { staffToken }
    ),
}
