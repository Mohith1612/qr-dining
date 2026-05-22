import { plansApi, type Plan } from "@/lib/api/plans"

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
    <div
      style={{
        minHeight: "100vh",
        background: "var(--color-bg)",
        padding: "48px 24px",
      }}
    >
      <div style={{ maxWidth: "900px", margin: "0 auto" }}>
        {/* Header */}
        <div style={{ textAlign: "center", marginBottom: "48px" }}>
          <h1
            style={{
              fontSize: "clamp(24px, 4vw, 36px)",
              fontWeight: 700,
              color: "var(--color-text)",
              margin: "0 0 12px",
              letterSpacing: "-0.02em",
            }}
          >
            Simple, transparent pricing
          </h1>
          <p style={{ fontSize: "16px", color: "var(--color-text-muted)", margin: 0 }}>
            Start for free. Upgrade as your restaurant grows.
          </p>
        </div>

        {/* Plan cards */}
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "repeat(auto-fit, minmax(240px, 1fr))",
            gap: "20px",
            marginBottom: "48px",
          }}
        >
          {plans.map(plan => {
            const isPopular = plan.tier === "standard"
            return (
              <div
                key={plan.id}
                style={{
                  background: "var(--color-surface)",
                  border: isPopular
                    ? "2px solid var(--color-accent)"
                    : "1px solid var(--color-border)",
                  borderRadius: "var(--radius-lg)",
                  padding: "28px 24px",
                  position: "relative",
                  boxShadow: isPopular ? "var(--shadow-card)" : "none",
                }}
              >
                {isPopular && (
                  <div
                    style={{
                      position: "absolute",
                      top: "-12px",
                      left: "50%",
                      transform: "translateX(-50%)",
                      background: "var(--color-accent)",
                      color: "var(--color-accent-fg)",
                      fontSize: "11px",
                      fontWeight: 700,
                      letterSpacing: "0.06em",
                      padding: "3px 12px",
                      borderRadius: "100px",
                      textTransform: "uppercase",
                    }}
                  >
                    Most popular
                  </div>
                )}

                <h2 style={{ margin: "0 0 6px", fontSize: "18px", fontWeight: 700, color: "var(--color-text)" }}>
                  {plan.name}
                </h2>

                <div style={{ marginBottom: "20px" }}>
                  <span style={{ fontSize: "32px", fontWeight: 700, color: "var(--color-text)" }}>
                    {plan.price_monthly === 0 ? "Free" : `₹${plan.price_monthly}`}
                  </span>
                  {plan.price_monthly > 0 && (
                    <span style={{ fontSize: "14px", color: "var(--color-text-muted)", marginLeft: "4px" }}>
                      / month
                    </span>
                  )}
                </div>

                {/* Feature rows */}
                <div style={{ display: "flex", flexDirection: "column", gap: "10px", marginBottom: "24px" }}>
                  {FEATURE_ROWS.map(row => {
                    const val = plan.features_json[row.key]
                    const display = formatFeatureValue(row.key, val)
                    const active = typeof val === "boolean" ? val : true
                    return (
                      <div
                        key={row.key}
                        style={{
                          display: "flex",
                          justifyContent: "space-between",
                          alignItems: "center",
                          fontSize: "14px",
                        }}
                      >
                        <span style={{ color: "var(--color-text-muted)" }}>{row.label}</span>
                        <span
                          style={{
                            fontWeight: 500,
                            color: active ? "var(--color-text)" : "var(--color-text-muted)",
                          }}
                        >
                          {display}
                        </span>
                      </div>
                    )
                  })}
                </div>

                <a
                  href="mailto:hello@qrdining.app"
                  style={{
                    display: "block",
                    textAlign: "center",
                    padding: "10px 0",
                    borderRadius: "var(--radius-base)",
                    border: isPopular ? "none" : "1px solid var(--color-border)",
                    background: isPopular ? "var(--color-accent)" : "transparent",
                    color: isPopular ? "var(--color-accent-fg)" : "var(--color-text)",
                    fontSize: "14px",
                    fontWeight: 600,
                    textDecoration: "none",
                    cursor: "pointer",
                  }}
                >
                  {plan.tier === "free" ? "Get started" : "Contact us"}
                </a>
              </div>
            )
          })}
        </div>

        <p style={{ textAlign: "center", fontSize: "13px", color: "var(--color-text-muted)" }}>
          Need a custom plan? <a href="mailto:hello@qrdining.app" style={{ color: "var(--color-accent)" }}>Get in touch.</a>
        </p>
      </div>
    </div>
  )
}
