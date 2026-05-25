import { cn } from "@/lib/utils"
import { plansApi, type Plan } from "@/lib/api/plans"
import { HospitalityCard } from "@/components/shared/HospitalityCard"

const FEATURE_ROWS: { label: string; key: keyof Plan["features_json"] }[] = [
  { label: "Branches",              key: "max_branches" },
  { label: "Tables",                key: "max_tables"   },
  { label: "Operational analytics", key: "analytics"    },
  { label: "Multi-branch analytics",key: "multi_branch" },
]

function formatFeatureValue(key: keyof Plan["features_json"], val: number | boolean): string {
  if (typeof val === "boolean") return val ? "✓" : "—"
  if (val === -1) return "Unlimited"
  return String(val)
}

function FeatureRow({ label, value, active }: { label: string; value: string; active: boolean }) {
  return (
    <div className="flex justify-between items-center">
      <span className="text-[13px] text-[var(--ink-3)]">{label}</span>
      <span className={cn("text-[13px] font-semibold", active ? "text-[var(--ink-1)]" : "text-[var(--ink-4)]")}>
        {value}
      </span>
    </div>
  )
}

function PlanCard({ plan }: { plan: Plan }) {
  const isPopular = plan.tier === "standard"

  return (
    <div className="relative">
      {isPopular && (
        <div className="absolute -top-3 left-1/2 -translate-x-1/2 z-10">
          <span className="eyebrow bg-[var(--accent)] text-[var(--accent-ink)] px-3 py-0.5 rounded-full text-[10px]">
            Most popular
          </span>
        </div>
      )}
      <HospitalityCard
        elev={isPopular ? 3 : 1}
        className="p-7 h-full"
        style={isPopular ? { border: "2px solid var(--accent)" } : undefined}
      >
        <p className="eyebrow mb-2">{plan.tier}</p>
        <h2 className="serif text-2xl font-medium text-[var(--ink-1)] mb-3">{plan.name}</h2>

        <div className="mb-5">
          <span className="serif text-4xl font-semibold text-[var(--ink-1)]">
            {plan.price_monthly === 0 ? "₹0" : `₹${plan.price_monthly}`}
          </span>
          <span className="text-[13px] text-[var(--ink-3)] ml-1">/ mo</span>
        </div>

        <div className="rule mb-5" />

        <div className="flex flex-col gap-3 mb-6">
          {FEATURE_ROWS.map((row) => {
            const val = plan.features_json[row.key]
            const active = typeof val === "boolean" ? val : true
            return (
              <FeatureRow
                key={row.key}
                label={row.label}
                value={formatFeatureValue(row.key, val)}
                active={active}
              />
            )
          })}
        </div>

        <a
          href="mailto:hello@qrdining.app"
          className={cn(
            "press block text-center py-3 rounded-[var(--rad-md)] text-[14px] font-semibold no-underline",
            isPopular
              ? "bg-[var(--accent)] text-[var(--accent-ink)]"
              : "border border-[var(--line-2)] text-[var(--ink-2)] bg-transparent"
          )}
        >
          {plan.tier === "free" ? "Get started" : "Contact us"}
        </a>
      </HospitalityCard>
    </div>
  )
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
    <div
      className="atmos min-h-screen bg-[var(--bg-base)] text-[var(--ink-1)]"
      style={{ backgroundImage: "var(--glow-warm)" }}
    >
      <div className="max-w-[1100px] mx-auto px-6 py-14">
        <div className="mb-12">
          <p className="eyebrow mb-2.5">Pricing</p>
          <h1 className="serif text-5xl font-medium text-[var(--ink-1)] mb-4 leading-[1.1]">
            Hospitality, on every plate.
          </h1>
          <p className="text-[15px] text-[var(--ink-2)] max-w-[480px] leading-relaxed">
            Start free and upgrade as your restaurant grows. All plans include realtime collaborative sessions, QR table management, and instant ordering.
          </p>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {plans.map((plan) => (
            <PlanCard key={plan.id} plan={plan} />
          ))}
        </div>

        <p className="text-center mt-12 text-[13px] text-[var(--ink-3)]">
          Need a custom plan?{" "}
          <a href="mailto:hello@qrdining.app" className="text-[var(--accent)] no-underline">
            Get in touch.
          </a>
        </p>
      </div>
    </div>
  )
}
