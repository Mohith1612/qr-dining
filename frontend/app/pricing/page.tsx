import { plansApi, type Plan } from "@/lib/api/plans"
import { HospitalityCard } from "@/components/shared/HospitalityCard"

const FEATURE_ROWS: { label: string; key: keyof Plan["features_json"] }[] = [
  { label: "Branches", key: "max_branches" },
  { label: "Tables", key: "max_tables" },
  { label: "Operational analytics", key: "analytics" },
  { label: "Multi-branch analytics", key: "multi_branch" },
]

function formatFeatureValue(key: keyof Plan["features_json"], val: number | boolean): string {
  if (typeof val === "boolean") return val ? "✓" : "—"
  if (val === -1) return "Unlimited"
  return String(val)
}

async function getPlans(): Promise<Plan[]> {
  try {
    const data = await plansApi.listPlans()
    return data.plans
  } catch {
    return []
  }
}

export default async function PricingPage() {
  const plans = await getPlans()

  return (
    <div className="min-h-screen px-6 py-12" style={{ background: "var(--color-bg)" }}>
      <div className="max-w-4xl mx-auto">
        {/* Header */}
        <div className="text-center mb-12">
          <h1
            className="text-[clamp(24px,4vw,36px)] font-bold mb-3 tracking-tight font-[family-name:var(--font-display)]"
            style={{ color: "var(--color-text)" }}
          >
            Simple, transparent pricing
          </h1>
          <p className="text-base" style={{ color: "var(--color-text-muted)" }}>
            Start for free. Upgrade as your restaurant grows.
          </p>
        </div>

        {/* Plan cards */}
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5 mb-12">
          {plans.map(plan => {
            const isPopular = plan.tier === "standard"
            return (
              <HospitalityCard
                key={plan.id}
                variant={isPopular ? "elevated" : "default"}
                className="relative"
                style={isPopular ? { border: "2px solid var(--color-accent)" } : undefined}
              >
                {isPopular && (
                  <div
                    className="absolute -top-3 left-1/2 -translate-x-1/2 text-[11px] font-bold tracking-[0.06em] px-3 py-0.5 rounded-full uppercase"
                    style={{ background: "var(--color-accent)", color: "var(--color-accent-fg)" }}
                  >
                    Most popular
                  </div>
                )}

                <h2
                  className="text-lg font-bold mb-1.5 font-[family-name:var(--font-display)]"
                  style={{ color: "var(--color-text)" }}
                >
                  {plan.name}
                </h2>

                <div className="mb-5">
                  <span className="text-[32px] font-bold" style={{ color: "var(--color-text)" }}>
                    {plan.price_monthly === 0 ? "Free" : `₹${plan.price_monthly}`}
                  </span>
                  {plan.price_monthly > 0 && (
                    <span className="text-sm ml-1" style={{ color: "var(--color-text-muted)" }}>
                      / month
                    </span>
                  )}
                </div>

                {/* Feature rows */}
                <div className="flex flex-col gap-2.5 mb-6">
                  {FEATURE_ROWS.map(row => {
                    const val = plan.features_json[row.key]
                    const display = formatFeatureValue(row.key, val)
                    const active = typeof val === "boolean" ? val : true
                    return (
                      <div key={row.key} className="flex justify-between items-center text-sm">
                        <span style={{ color: "var(--color-text-muted)" }}>{row.label}</span>
                        <span
                          className="font-medium"
                          style={{ color: active ? "var(--color-text)" : "var(--color-text-muted)" }}
                        >
                          {display}
                        </span>
                      </div>
                    )
                  })}
                </div>

                <a
                  href="mailto:hello@qrdining.app"
                  className="block text-center py-2.5 rounded-[var(--radius-base)] text-sm font-semibold no-underline"
                  style={
                    isPopular
                      ? { background: "var(--color-accent)", color: "var(--color-accent-fg)" }
                      : { border: "1px solid var(--color-border)", color: "var(--color-text)" }
                  }
                >
                  {plan.tier === "free" ? "Get started" : "Contact us"}
                </a>
              </HospitalityCard>
            )
          })}
        </div>

        <p className="text-center text-[13px]" style={{ color: "var(--color-text-muted)" }}>
          Need a custom plan?{" "}
          <a href="mailto:hello@qrdining.app" style={{ color: "var(--color-accent)" }}>
            Get in touch.
          </a>
        </p>
      </div>
    </div>
  )
}
