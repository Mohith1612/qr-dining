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
      className="atmos min-h-screen"
      style={{ background: "var(--bg-base)", color: "var(--ink-1)", backgroundImage: "var(--glow-warm)" }}
    >
      <div style={{ maxWidth: 1100, margin: "0 auto", padding: "56px 24px 64px" }}>
        {/* Header */}
        <div style={{ marginBottom: 48 }}>
          <p className="eyebrow" style={{ marginBottom: 10 }}>Pricing</p>
          <h1 className="serif" style={{ fontSize: 48, fontWeight: 500, color: "var(--ink-1)", margin: "0 0 16px", lineHeight: 1.1 }}>
            Hospitality, on every plate.
          </h1>
          <p style={{ fontSize: 15, color: "var(--ink-2)", maxWidth: 480, lineHeight: 1.6, margin: 0 }}>
            Start free and upgrade as your restaurant grows. All plans include realtime collaborative sessions, QR table management, and instant ordering.
          </p>
        </div>

        {/* Plan cards */}
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {plans.map((plan) => {
            const isPopular = plan.tier === "standard"
            return (
              <div key={plan.id} style={{ position: "relative" }}>
                {isPopular && (
                  <div style={{
                    position: "absolute", top: -12, left: "50%", transform: "translateX(-50%)",
                    zIndex: 1,
                  }}>
                    <span className="eyebrow" style={{
                      background: "var(--accent)", color: "var(--accent-ink)",
                      padding: "3px 12px", borderRadius: "var(--rad-pill)",
                      fontSize: 10,
                    }}>
                      Most popular
                    </span>
                  </div>
                )}
                <HospitalityCard
                  elev={isPopular ? 3 : 1}
                  style={{
                    padding: "28px 24px",
                    border: isPopular ? "2px solid var(--accent)" : undefined,
                    height: "100%",
                  }}
                >
                  {/* Tier */}
                  <p className="eyebrow" style={{ marginBottom: 8 }}>{plan.tier}</p>

                  {/* Plan name */}
                  <h2 className="serif" style={{ fontSize: 24, fontWeight: 500, color: "var(--ink-1)", margin: "0 0 12px" }}>
                    {plan.name}
                  </h2>

                  {/* Price */}
                  <div style={{ marginBottom: 20 }}>
                    <span className="serif" style={{ fontSize: 36, fontWeight: 600, color: "var(--ink-1)" }}>
                      {plan.price_monthly === 0 ? "₹0" : `₹${plan.price_monthly}`}
                    </span>
                    <span style={{ fontSize: 13, color: "var(--ink-3)", marginLeft: 4 }}>/ mo</span>
                  </div>

                  {/* Divider */}
                  <div className="rule" style={{ marginBottom: 20 }} />

                  {/* Feature rows */}
                  <div style={{ display: "flex", flexDirection: "column", gap: 12, marginBottom: 24 }}>
                    {FEATURE_ROWS.map((row) => {
                      const val = plan.features_json[row.key]
                      const display = formatFeatureValue(row.key, val)
                      const active = typeof val === "boolean" ? val : true
                      return (
                        <div key={row.key} style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                          <span style={{ fontSize: 13, color: "var(--ink-3)" }}>{row.label}</span>
                          <span style={{
                            fontSize: 13, fontWeight: 600,
                            color: active ? "var(--ink-1)" : "var(--ink-4)",
                          }}>
                            {display}
                          </span>
                        </div>
                      )
                    })}
                  </div>

                  {/* CTA */}
                  <a
                    href="mailto:hello@qrdining.app"
                    className="press"
                    style={{
                      display: "block", textAlign: "center",
                      padding: "12px 0",
                      borderRadius: "var(--rad-md)",
                      fontSize: 14, fontWeight: 600,
                      textDecoration: "none",
                      ...(isPopular
                        ? { background: "var(--accent)", color: "var(--accent-ink)" }
                        : { border: "1px solid var(--line-2)", color: "var(--ink-2)", background: "transparent" }
                      ),
                    }}
                  >
                    {plan.tier === "free" ? "Get started" : "Contact us"}
                  </a>
                </HospitalityCard>
              </div>
            )
          })}
        </div>

        {/* Footer note */}
        <p style={{ textAlign: "center", marginTop: 48, fontSize: 13, color: "var(--ink-3)" }}>
          Need a custom plan?{" "}
          <a href="mailto:hello@qrdining.app" style={{ color: "var(--accent)", textDecoration: "none" }}>
            Get in touch.
          </a>
        </p>
      </div>
    </div>
  )
}
