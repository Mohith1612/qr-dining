"use client"

import { useState } from "react"
import { cn } from "@/lib/utils"
import { PerformanceTab } from "@/components/staff-performance/PerformanceTab"
import { LoyaltyTab } from "@/components/loyalty/LoyaltyTab"
import { SessionsTab } from "@/components/admin/tabs/SessionsTab"
import { MenuTab } from "@/components/admin/tabs/MenuTab"
import { TablesTab } from "@/components/admin/tabs/TablesTab"
import { CollateralTab } from "@/components/admin/tabs/CollateralTab"
import { StaffTab } from "@/components/admin/tabs/StaffTab"
import { StatsTab } from "@/components/admin/tabs/StatsTab"
import { PlanTab } from "@/components/admin/tabs/PlanTab"
import { PromosTab } from "@/components/admin/tabs/PromosTab"
import { AppearanceTab } from "@/components/admin/tabs/AppearanceTab"
import { SettingsTab } from "@/components/admin/tabs/SettingsTab"

// ─── Page ────────────────────────────────────────────────────────────────────

const TABS = [
  { id: "sessions",    label: "Sessions"    },
  { id: "menu",        label: "Menu"        },
  { id: "tables",      label: "Tables"      },
  { id: "collateral",  label: "Collateral"  },
  { id: "staff",       label: "Staff"       },
  { id: "stats",       label: "Stats"       },
  { id: "performance", label: "Performance" },
  { id: "loyalty",     label: "Loyalty"     },
  { id: "plan",        label: "Plan"        },
  { id: "promos",      label: "Promos"      },
  { id: "appearance",  label: "Appearance"  },
  { id: "settings",    label: "Settings"    },
]

export default function AdminPage() {
  const [activeTab, setActiveTab] = useState("sessions")

  return (
    <div className="screen-enter bg-[var(--bg-base)] text-[var(--ink-1)] min-h-screen px-5 py-6">
      {/* Heading */}
      <p className="eyebrow">Operations</p>
      <h1 className="display-xl mt-1.5">House overview</h1>

      {/* Pill tab switcher */}
      <div className="hscroll mt-6 overflow-x-auto pb-0.5">
        <div className="inline-flex gap-0.5 p-[3px] bg-[var(--bg-elev-1)] rounded-full border border-[var(--line-1)]">
          {TABS.map(({ id, label }) => (
            <button
              key={id}
              onClick={() => setActiveTab(id)}
              className={cn(
                "press px-[18px] py-2 rounded-full border-0 text-[13px] cursor-pointer whitespace-nowrap transition-[background,color,box-shadow] duration-[var(--dur-fast)]",
                activeTab === id
                  ? "font-semibold bg-[var(--bg-elev-3)] text-[var(--ink-1)] shadow-[var(--shadow-1)]"
                  : "font-normal bg-transparent text-[var(--ink-3)]"
              )}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      {/* Tab content */}
      <div className="screen-enter mt-5">
        {activeTab === "sessions"   && <SessionsTab />}
        {activeTab === "menu"       && <MenuTab />}
        {activeTab === "tables"     && <TablesTab />}
        {activeTab === "collateral" && <CollateralTab />}
        {activeTab === "staff"      && <StaffTab />}
        {activeTab === "stats"      && <StatsTab />}
        {activeTab === "performance" && <PerformanceTab />}
        {activeTab === "loyalty"    && <LoyaltyTab />}
        {activeTab === "plan"       && <PlanTab />}
        {activeTab === "promos"     && <PromosTab />}
        {activeTab === "appearance" && <AppearanceTab />}
        {activeTab === "settings"   && <SettingsTab />}
      </div>
    </div>
  )
}
