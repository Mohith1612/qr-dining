"use client"

import { useEffect, useMemo, useRef, useState } from "react"
import { useMenuStore } from "@/store/menu"
import { useCart } from "@/hooks/useCart"
import { useSession } from "@/hooks/useSession"
import { menuApi } from "@/lib/api/menu"
import { ApiError, friendlyErrorMessage } from "@/lib/api/client"
import { formatCurrency } from "@/lib/format"
import { MenuSkeleton } from "@/components/shared/LoadingSkeleton"
import { BottomSheet } from "@/components/shared/BottomSheet"
import { Vignette } from "@/components/shared/Vignette"
import { FeaturedCarousel } from "@/components/shared/FeaturedCarousel"
import { FloatingCart } from "@/components/shared/FloatingCart"
import { DietaryTag, BadgeTag, SpiceIndicator } from "@/components/shared/MetaTag"
import { Minus, Plus, Search } from "lucide-react"
import { toast } from "sonner"
import { useRouter, useSearchParams } from "next/navigation"
import { use } from "react"
import { cn, groupBy } from "@/lib/utils"
import { BeverageModifierGroup, isBeverageCategory } from "@/components/shared/BeverageModifierGrid"
import { SearchBar } from "@/components/shared/SearchBar"
import { EmptyState } from "@/components/shared/EmptyState"
import { useFilteredMenu } from "@/hooks/useFilteredMenu"
import type { MenuItem, ItemModifier, DietaryFlag, ItemBadge } from "@/types/api"

interface SheetState {
  item: MenuItem
  quantity: number
  selectedModifiers: number[]
  note: string
  isBeverage: boolean
}

interface Props {
  params: Promise<{ id: string }>
}

interface ItemRowProps {
  item: MenuItem
  qty: number
  onTap: (item: MenuItem) => void
}

function itemHue(id: number): number {
  return (id * 47 + 15) % 60 + 20
}

function ItemRow({ item, qty, onTap }: ItemRowProps) {
  const [imgError, setImgError] = useState(false)
  return (
    <button
      onClick={() => item.is_available && onTap(item)}
      disabled={!item.is_available}
      className="press"
      style={{
        border: 0, background: "transparent", padding: "18px 0",
        display: "flex", gap: 16, alignItems: "flex-start", textAlign: "left",
        width: "100%", opacity: item.is_available ? 1 : 0.4,
      }}
      aria-disabled={!item.is_available}
    >
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ display: "flex", alignItems: "baseline", gap: 8 }}>
          <span className="serif" style={{ fontSize: 20, fontWeight: 500, letterSpacing: "-0.015em", color: "var(--ink-1)", lineHeight: 1.15 }}>
            {item.name}
          </span>
          <span className="leader" />
          <span className="serif" style={{ fontSize: 17, color: "var(--accent)", fontWeight: 500, whiteSpace: "nowrap", fontVariantNumeric: "tabular-nums" }}>
            {formatCurrency(item.price)}
          </span>
        </div>
        {item.description && (
          <div style={{ color: "var(--ink-3)", fontSize: 13, lineHeight: 1.6, marginTop: 5 }}>
            {item.description}
          </div>
        )}
        {(item.dietary_flags?.length || item.item_badges?.length || !!item.spice_level) ? (
          <div style={{ display: "flex", flexWrap: "wrap", gap: 4, marginTop: 6 }}>
            {item.dietary_flags?.map((f) => <DietaryTag key={f} flag={f} />)}
            {item.item_badges?.map((b) => <BadgeTag key={b} badge={b} />)}
            {item.spice_level ? <SpiceIndicator level={item.spice_level} /> : null}
          </div>
        ) : null}
        {!item.is_available && (
          <div style={{ color: "var(--ink-4)", fontSize: 11, marginTop: 5, letterSpacing: "0.05em", textTransform: "uppercase" }}>
            Not available
          </div>
        )}
      </div>
      <div style={{ position: "relative", flexShrink: 0 }}>
        {item.image_url && !imgError ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={item.image_url}
            alt=""
            aria-hidden
            loading="lazy"
            onError={() => setImgError(true)}
            style={{
              width: 66, height: 66, borderRadius: 12,
              objectFit: "cover", flexShrink: 0,
              outline: qty > 0 ? "2px solid var(--accent)" : "none",
              outlineOffset: 2,
            }}
          />
        ) : (
          <Vignette hue={itemHue(item.id)} size={66} ring={qty > 0} />
        )}
        {qty > 0 && (
          <span style={{
            position: "absolute", bottom: -3, right: -3,
            minWidth: 22, height: 22, borderRadius: 999, padding: "0 7px",
            background: "var(--accent)", color: "var(--accent-ink)",
            border: "2px solid var(--bg-base)",
            display: "inline-flex", alignItems: "center", justifyContent: "center",
            fontSize: 11, fontWeight: 700, letterSpacing: "-0.005em",
            fontVariantNumeric: "tabular-nums",
            boxShadow: "0 4px 12px -4px rgba(0,0,0,0.5)",
            animation: "pop 0.32s var(--ease-back)",
          }}>
            {qty}
          </span>
        )}
      </div>
    </button>
  )
}

const DIETARY_FILTERS: { flag: DietaryFlag; label: string }[] = [
  { flag: "vegetarian", label: "Veg" },
  { flag: "vegan", label: "Vegan" },
  { flag: "non-veg", label: "Non-Veg" },
  { flag: "jain", label: "Jain" },
  { flag: "egg", label: "Egg" },
]

const BADGE_FILTERS: { badge: ItemBadge; label: string }[] = [
  { badge: "chef-special", label: "Chef Special" },
  { badge: "bestseller", label: "Bestseller" },
  { badge: "new", label: "New" },
]

interface FilterBarProps {
  activeFilters: { dietary: DietaryFlag[]; badges: ItemBadge[]; spice: number | null }
  hasActiveFilters: boolean
  onDietary: (flag: DietaryFlag) => void
  onBadge: (badge: ItemBadge) => void
  onSpice: (level: number) => void
  onClear: () => void
}

function FilterPill({
  active, onClick, children,
}: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      onClick={onClick}
      className="press"
      style={{
        flexShrink: 0,
        fontSize: 12, fontWeight: 600,
        padding: "6px 13px",
        borderRadius: "var(--rad-pill)",
        border: "1px solid",
        borderColor: active ? "var(--accent)" : "var(--line-2)",
        background: active ? "var(--accent-soft)" : "var(--bg-elev-1)",
        color: active ? "var(--accent)" : "var(--ink-3)",
        transition: "background var(--dur-fast) var(--ease), color var(--dur-fast) var(--ease), border-color var(--dur-fast) var(--ease)",
        cursor: "pointer",
        whiteSpace: "nowrap",
      }}
    >
      {children}
    </button>
  )
}

function FilterDivider() {
  return (
    <span style={{
      flexShrink: 0,
      width: 1, height: 20, alignSelf: "center",
      background: "var(--line-2)",
      marginLeft: 4,
    }} />
  )
}

function FilterBar({ activeFilters, hasActiveFilters, onDietary, onBadge, onSpice, onClear }: FilterBarProps) {
  return (
    <div style={{ padding: "10px 0 12px", borderBottom: "1px solid var(--line-1)" }}>
      <div className="relative">
        <div className="hscroll flex gap-1.5 px-5 items-center">
          <FilterPill active={!hasActiveFilters} onClick={onClear}>All</FilterPill>
          <FilterDivider />
          {DIETARY_FILTERS.map(({ flag, label }) => (
            <FilterPill
              key={flag}
              active={activeFilters.dietary.includes(flag)}
              onClick={() => onDietary(flag)}
            >
              {label}
            </FilterPill>
          ))}
          <FilterDivider />
          {BADGE_FILTERS.map(({ badge, label }) => (
            <FilterPill
              key={badge}
              active={activeFilters.badges.includes(badge)}
              onClick={() => onBadge(badge)}
            >
              {label}
            </FilterPill>
          ))}
          <FilterDivider />
          {[1, 2, 3].map((level) => (
            <FilterPill
              key={level}
              active={activeFilters.spice === level}
              onClick={() => onSpice(level)}
            >
              {"🌶".repeat(level)}
            </FilterPill>
          ))}
        </div>
        <div style={{
          position: "absolute", right: 0, top: 0, bottom: 0, width: 32, pointerEvents: "none",
          background: "linear-gradient(to right, transparent, var(--bg-base))",
        }} />
      </div>
    </div>
  )
}

export default function MenuPage({ params }: Props) {
  const { id: sessionId } = use(params)
  const router = useRouter()
  const searchParams = useSearchParams()
  const featured = useMenuStore((s) => s.featured)
  const categories = useMenuStore((s) => s.categories)
  const menuLoading = useMenuStore((s) => s.loading)
  const activeFilters = useMenuStore((s) => s.activeFilters)
  const setActiveFilters = useMenuStore((s) => s.setActiveFilters)
  const clearFilters = useMenuStore((s) => s.clearFilters)
  const isSearchMode = useMenuStore((s) => s.isSearchMode)
  const setSearchMode = useMenuStore((s) => s.setSearchMode)
  const searchQuery = useMenuStore((s) => s.searchQuery)
  const setSearchQuery = useMenuStore((s) => s.setSearchQuery)
  const { categories: filteredCategories, isFiltered } = useFilteredMenu()
  const { session } = useSession()
  const { items: cartItems, itemCount, addItem } = useCart()
  const [activeCatId, setActiveCatId] = useState<number | null>(null)
  const [sheet, setSheet] = useState<SheetState | null>(null)
  const [adding, setAdding] = useState(false)
  const [prefersReducedMotion, setPrefersReducedMotion] = useState(false)

  const scrollContainerRef = useRef<HTMLDivElement>(null)
  const sectionRefs = useRef<(HTMLElement | null)[]>([])
  const activePillRef = useRef<HTMLButtonElement | null>(null)
  const observerRef = useRef<IntersectionObserver | null>(null)
  const isScrollingRef = useRef(false)
  const scrollTimeoutRef = useRef<ReturnType<typeof setTimeout>>(undefined)

  useEffect(() => {
    setPrefersReducedMotion(window.matchMedia("(prefers-reduced-motion: reduce)").matches)
    // Use Shell's scrollable main — the root layout also renders a <main>, so we
    // must target the inner one by its overflow class, not just "main".
    const main = document.querySelector("main.overflow-y-auto") as HTMLDivElement | null
    if (main) scrollContainerRef.current = main
  }, [])

  useEffect(() => {
    if (!session?.branch_id || categories.length > 0) return
    useMenuStore.getState().setLoading(true)
    menuApi
      .getMenu(session.branch_id)
      .then((data) => useMenuStore.getState().setMenu(data))
      .catch(() => useMenuStore.getState().setError("Failed to load menu"))
      .finally(() => useMenuStore.getState().setLoading(false))
  }, [session?.branch_id, categories.length])

  useEffect(() => {
    if (categories.length > 0 && activeCatId === null) {
      setActiveCatId(categories[0].id)
    }
  }, [categories.length, activeCatId])

  useEffect(() => {
    const container = scrollContainerRef.current
    if (categories.length === 0 || !container) return

    observerRef.current?.disconnect()
    observerRef.current = new IntersectionObserver(
      (entries) => {
        if (isScrollingRef.current) return
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            const idx = sectionRefs.current.indexOf(entry.target as HTMLElement)
            if (idx !== -1) setActiveCatId(categories[idx].id)
          }
        })
      },
      { root: container, rootMargin: "-20% 0px -70% 0px" }
    )

    sectionRefs.current.forEach((el) => {
      if (el) observerRef.current!.observe(el)
    })

    return () => observerRef.current?.disconnect()
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [categories])

  useEffect(() => {
    // Skip during programmatic scrolls — scrollIntoView would fight container.scrollTo()
    if (!isScrollingRef.current) {
      activePillRef.current?.scrollIntoView({ behavior: "smooth", block: "nearest", inline: "center" })
    }
  }, [activeCatId])

  useEffect(() => {
    return () => {
      useMenuStore.getState().setSearchMode(false)
      useMenuStore.getState().setSearchQuery("")
    }
  }, [])

  function scrollToCategory(catId: number) {
    const idx = categories.findIndex((c) => c.id === catId)
    const el = sectionRefs.current[idx]
    const container = scrollContainerRef.current
    if (!el || !container) return

    // Immediate pill feedback — observer only fires on intersection changes,
    // so if scroll lands in a stable zone the callback never re-fires.
    setActiveCatId(catId)

    const pillBar = document.getElementById("category-pill-bar")
    const offset = (pillBar?.offsetHeight ?? 56) + 8
    const top = el.getBoundingClientRect().top - container.getBoundingClientRect().top + container.scrollTop - offset

    isScrollingRef.current = true
    clearTimeout(scrollTimeoutRef.current)
    // 600ms covers typical smooth-scroll duration; observer resumes after.
    scrollTimeoutRef.current = setTimeout(() => { isScrollingRef.current = false }, 600)

    container.scrollTo({ top, behavior: prefersReducedMotion ? "auto" : "smooth" })
  }

  function openSheet(item: MenuItem, categoryName?: string) {
    setSheet({ item, quantity: 1, selectedModifiers: [], note: "", isBeverage: isBeverageCategory(categoryName ?? "") })
  }

  // Deep-link from the landing's specials/popular: ?item=<id> opens its sheet once.
  const itemParam = searchParams.get("item")
  useEffect(() => {
    if (!itemParam || categories.length === 0) return
    const targetId = Number(itemParam)
    for (const cat of categories) {
      const found = cat.items.find((i) => i.id === targetId)
      if (found) { openSheet(found, cat.name); break }
    }
    router.replace(`/session/${sessionId}/menu`, { scroll: false })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [itemParam, categories.length])

  function toggleModifier(id: number) {
    setSheet((s) => {
      if (!s) return s
      if (s.selectedModifiers.includes(id)) {
        return { ...s, selectedModifiers: s.selectedModifiers.filter((m) => m !== id) }
      }
      const mod = s.item.modifiers?.find((m) => m.id === id)
      const group = mod?.modifier_group ?? ""
      // Single-select: any modifier in this group flagged single_select makes
      // the whole group exclusive — picking one clears its siblings.
      const isSingleSelect =
        !!group && (s.item.modifiers ?? []).some((m) => (m.modifier_group ?? "") === group && m.single_select)
      const groupIds = isSingleSelect
        ? new Set((s.item.modifiers ?? []).filter((m) => (m.modifier_group ?? "") === group).map((m) => m.id))
        : new Set<number>()
      const kept = s.selectedModifiers.filter((m) => !groupIds.has(m))
      return { ...s, selectedModifiers: [...kept, id] }
    })
  }

  async function handleAddToCart() {
    if (!sheet) return
    setAdding(true)
    try {
      await addItem(sheet.item.id, sheet.quantity, sheet.selectedModifiers, sheet.note || undefined)
      toast.success(`${sheet.item.name} added`)
      setSheet(null)
    } catch (err) {
      toast.error(err instanceof ApiError ? friendlyErrorMessage(err.code) : "Couldn't add item. Please try again.")
    } finally {
      setAdding(false)
    }
  }

  const cartCountByItem: Record<number, number> = {}
  for (const ci of cartItems) {
    cartCountByItem[ci.menu_item_id] = (cartCountByItem[ci.menu_item_id] ?? 0) + ci.quantity
  }

  const cartTotal = cartItems.reduce((s, ci) => {
    const modTotal = ci.selected_modifiers.reduce((m, mod) => m + mod.price_delta, 0)
    return s + ((ci.item_price ?? 0) + modTotal) * ci.quantity
  }, 0)
  const requiredGroupsFulfilled = useMemo(() => {
    if (!sheet?.isBeverage) return true
    const requiredGroups = [...new Set(
      sheet.item.modifiers?.filter((m) => m.is_required).map((m) => m.modifier_group ?? "") ?? []
    )]
    return requiredGroups.every((group) =>
      sheet.item.modifiers?.filter((m) => (m.modifier_group ?? "") === group)
        .some((m) => sheet.selectedModifiers.includes(m.id))
    )
  }, [sheet])

  const hasActiveFilters =
    activeFilters.dietary.length > 0 || activeFilters.badges.length > 0 || activeFilters.spice !== null

  const totalDishes = categories.reduce((s, c) => s + c.items.length, 0)

  if (menuLoading) return <MenuSkeleton />

  return (
    <div className="screen-enter" style={{ background: "var(--bg-base)" }}>

      {/* Editorial header */}
      <div className="page-glow" style={{ padding: "24px 20px 16px" }}>
        <span className="eyebrow">The Carte</span>
        <h1 className="display-lg" style={{ margin: "6px 0 4px" }}>
          Tonight&apos;s menu
        </h1>
        <p style={{ margin: 0, color: "var(--ink-2)", fontSize: 13 }}>
          {totalDishes} dishes across {categories.length} sections
        </p>
      </div>

      {/* Search bar — search mode only, replaces carousel */}
      {isSearchMode ? (
        <SearchBar
          onClose={() => {
            setSearchMode(false)
            setSearchQuery("")
          }}
        />
      ) : (
        <FeaturedCarousel items={featured} onSelect={openSheet} />
      )}

      {/* Filter bar — scrolls away with content */}
      <FilterBar
        activeFilters={activeFilters}
        hasActiveFilters={hasActiveFilters}
        onDietary={(flag) => {
          const already = activeFilters.dietary.includes(flag)
          setActiveFilters({ dietary: already ? activeFilters.dietary.filter((f) => f !== flag) : [...activeFilters.dietary, flag] })
        }}
        onBadge={(badge) => {
          const already = activeFilters.badges.includes(badge)
          setActiveFilters({ badges: already ? activeFilters.badges.filter((b) => b !== badge) : [...activeFilters.badges, badge] })
        }}
        onSpice={(level) => setActiveFilters({ spice: activeFilters.spice === level ? null : level })}
        onClear={clearFilters}
      />

      {/* Category pills — sticky, hidden in search mode or when only one category */}
      {!isSearchMode && filteredCategories.length > 1 && (
        <div
          id="category-pill-bar"
          style={{
            position: "sticky",
            top: 0,
            zIndex: 10,
            background: "color-mix(in srgb, var(--bg-base) 92%, transparent)",
            backdropFilter: "blur(12px)",
            WebkitBackdropFilter: "blur(12px)",
            borderBottom: "1px solid var(--line-1)",
            padding: "10px 0 12px",
          }}
        >
          <div className="relative">
            <div className="hscroll flex gap-1.5 px-5">
              {categories.map((c) => {
                const active = c.id === activeCatId
                return (
                  <button
                    key={c.id}
                    id={`pill-${c.id}`}
                    ref={(el) => { if (active) activePillRef.current = el }}
                    onClick={() => scrollToCategory(c.id)}
                    aria-current={active ? "true" : undefined}
                    className={cn(
                      "press px-4 py-2.5 rounded-full border text-[13px] whitespace-nowrap transition-[background,color,border-color] duration-[var(--dur-fast)]",
                      active
                        ? "font-semibold border-[var(--accent)] bg-[var(--accent)] text-[var(--accent-ink)] shadow-[var(--shadow-2)]"
                        : "font-medium border-[var(--line-2)] bg-[var(--bg-elev-1)] text-[var(--ink-2)] shadow-[var(--shadow-1)]"
                    )}
                  >
                    {c.name}
                  </button>
                )
              })}
            </div>
            {/* Right fade mask */}
            <div style={{
              position: "absolute", right: 0, top: 0, bottom: 0, width: 32, pointerEvents: "none",
              background: "linear-gradient(to right, transparent, var(--bg-base))",
            }} />
          </div>
        </div>
      )}

      {/* All category sections */}
      {/* TODO: If item count > 80, consider react-window for virtualized list */}
      <div style={{ paddingBottom: itemCount > 0 ? 80 : 24 }}>
        {/* Results count when search or filter is active */}
        {isFiltered && filteredCategories.length > 0 && (
          <div
            aria-live="polite"
            className="eyebrow"
            style={{ padding: "8px 20px", color: "var(--ink-3)", fontSize: 12 }}
          >
            {(() => { const n = filteredCategories.reduce((s, c) => s + c.items.length, 0); return `${n} ${n === 1 ? "dish" : "dishes"} found` })()}
          </div>
        )}

        {isFiltered && filteredCategories.length === 0 ? (
          <EmptyState
            icon={Search}
            title={searchQuery ? `No dishes match "${searchQuery}"` : "No dishes match these filters"}
            description="Try a different search or browse by category"
            action={{ label: "Clear search", onClick: () => { setSearchQuery(""); clearFilters() } }}
          />
        ) : (
          filteredCategories.map((cat, idx) => (
            <section
              key={cat.id}
              id={`category-${cat.id}`}
              ref={(el) => { sectionRefs.current[idx] = el }}
              style={{ paddingBottom: 32 }}
            >
              {idx > 0 && (
                <div style={{ margin: "0 20px", borderTop: "1px solid var(--line-2)" }} />
              )}
              <div style={{ padding: "32px 20px 0" }}>
                <p className="eyebrow" style={{ color: "var(--ink-4)", margin: "0 0 4px" }}>
                  {cat.items.length} {cat.items.length === 1 ? "dish" : "dishes"}
                </p>
                <h2 className="serif" style={{ fontSize: 22, fontWeight: 500, color: "var(--ink-1)", margin: "0 0 12px", letterSpacing: "-0.01em" }}>
                  {cat.name}
                </h2>
                <hr className="rule" style={{ margin: 0 }} />
              </div>
              <div style={{ padding: "0 20px" }}>
                {cat.items.map((item, i, arr) => (
                  <div key={item.id}>
                    <ItemRow item={item} qty={cartCountByItem[item.id] ?? 0} onTap={(item) => openSheet(item, cat.name)} />
                    {i < arr.length - 1 && <hr className="rule" style={{ margin: 0 }} />}
                  </div>
                ))}
              </div>
            </section>
          ))
        )}
      </div>

      {/* Floating cart — glass pill above the bottom nav */}
      <FloatingCart
        itemCount={itemCount}
        total={formatCurrency(cartTotal)}
        onClick={() => router.push(`/session/${sessionId}/cart`)}
        label="View Order"
      />

      {/* Item bottom sheet */}
      <BottomSheet
        open={!!sheet}
        onClose={() => setSheet(null)}
        title={sheet?.item.name}
      >
        {sheet && (
          <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
            {/* Hero image */}
            <div style={{ height: 168, borderRadius: "var(--rad-lg)", overflow: "hidden", background: "var(--bg-sunken)", display: "flex", alignItems: "center", justifyContent: "center", marginTop: -2 }}>
              {sheet.item.image_url ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={sheet.item.image_url} alt="" aria-hidden style={{ width: "100%", height: "100%", objectFit: "cover" }} />
              ) : (
                <Vignette hue={itemHue(sheet.item.id)} size={96} />
              )}
            </div>

            {sheet.item.description && (
              <p style={{ margin: 0, color: "var(--ink-2)", fontSize: 14, lineHeight: 1.65 }}>
                {sheet.item.description}
              </p>
            )}

            {/* Price + stepper */}
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
              <span className="serif" style={{ fontSize: 26, fontWeight: 600, color: "var(--ink-1)", letterSpacing: "-0.02em" }}>
                {formatCurrency(sheet.item.price)}
              </span>
              <div style={{ display: "flex", alignItems: "center", gap: 14 }}>
                <button
                  onClick={() => setSheet((s) => s ? { ...s, quantity: Math.max(1, s.quantity - 1) } : s)}
                  style={{
                    width: 40, height: 40, borderRadius: 999,
                    border: "1px solid var(--line-2)", color: "var(--ink-1)",
                    background: "var(--bg-elev-2)",
                    display: "flex", alignItems: "center", justifyContent: "center",
                    transition: "background var(--dur-fast) var(--ease)",
                  }}
                  aria-label="Decrease quantity"
                >
                  <Minus style={{ width: 14, height: 14 }} aria-hidden />
                </button>
                <span style={{ width: 28, textAlign: "center", fontWeight: 600, fontSize: 17, color: "var(--ink-1)" }}>
                  {sheet.quantity}
                </span>
                <button
                  onClick={() => setSheet((s) => s ? { ...s, quantity: s.quantity + 1 } : s)}
                  style={{
                    width: 40, height: 40, borderRadius: 999,
                    background: "var(--accent)", color: "var(--accent-ink)",
                    border: "none",
                    display: "flex", alignItems: "center", justifyContent: "center",
                    transition: "opacity var(--dur-fast) var(--ease)",
                  }}
                  aria-label="Increase quantity"
                >
                  <Plus style={{ width: 14, height: 14 }} aria-hidden />
                </button>
              </div>
            </div>

            {/* Modifiers */}
            {sheet.item.modifiers && sheet.item.modifiers.length > 0 && (
              <div>
                <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 10 }}>
                  <span className="eyebrow">Customise</span>
                  {sheet.item.modifiers?.some((m) => m.is_required) && (
                    <span style={{ fontSize: 9.5, fontWeight: 700, letterSpacing: "0.08em", textTransform: "uppercase", color: "#735B25", background: "#F3E7C8", padding: "2px 7px", borderRadius: "var(--rad-pill)" }}>Required</span>
                  )}
                </div>
                {sheet.isBeverage ? (
                  <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
                    {Object.entries(groupBy(sheet.item.modifiers, (m) => m.modifier_group ?? "add-ons")).map(([group, mods]) => (
                      <BeverageModifierGroup
                        key={group}
                        groupName={group}
                        modifiers={mods}
                        selected={sheet.selectedModifiers}
                        onToggle={toggleModifier}
                      />
                    ))}
                  </div>
                ) : (
                  <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
                    {Object.entries(groupBy(sheet.item.modifiers, (m) => m.modifier_group ?? "")).map(([group, mods]) => {
                      // A group is exclusive (radio) when any of its modifiers
                      // carries single_select — same rule toggleModifier and the
                      // server (MODIFIER_CONFLICT) enforce.
                      const singleSelect = !!group && mods.some((m) => m.single_select)
                      return (
                        <div key={group || "add-ons"}>
                          {group && (
                            <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 6 }}>
                              <span style={{ fontSize: 11.5, fontWeight: 600, letterSpacing: "0.06em", textTransform: "uppercase", color: "var(--ink-3)" }}>
                                {group}
                              </span>
                              {singleSelect && (
                                <span style={{ fontSize: 9.5, fontWeight: 700, letterSpacing: "0.08em", textTransform: "uppercase", color: "var(--accent)", background: "var(--accent-soft)", padding: "2px 7px", borderRadius: "var(--rad-pill)" }}>
                                  Pick one
                                </span>
                              )}
                            </div>
                          )}
                          <div style={{ borderRadius: "var(--rad-lg)", overflow: "hidden", border: "1px solid var(--line-2)" }}>
                            {mods.map((mod: ItemModifier, i: number) => (
                              <label
                                key={mod.id}
                                style={{
                                  display: "flex", alignItems: "center", justifyContent: "space-between",
                                  padding: "14px 16px", cursor: "default",
                                  borderTop: i > 0 ? "1px solid var(--line-1)" : "none",
                                  background: sheet.selectedModifiers.includes(mod.id) ? "var(--accent-soft)" : "transparent",
                                  transition: "background var(--dur-fast) var(--ease)",
                                }}
                              >
                                <span style={{ fontSize: 14, color: "var(--ink-1)" }}>{mod.name}</span>
                                <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                                  {mod.price_delta !== 0 && (
                                    <span style={{ fontSize: 12, color: "var(--ink-3)" }}>
                                      +{formatCurrency(mod.price_delta)}
                                    </span>
                                  )}
                                  <input
                                    type={singleSelect ? "radio" : "checkbox"}
                                    name={singleSelect ? `modifier-group-${group}` : undefined}
                                    checked={sheet.selectedModifiers.includes(mod.id)}
                                    // Radios don't fire change when the checked one is
                                    // clicked again, so exclusive groups toggle on click
                                    // (toggleModifier handles deselect + sibling clearing).
                                    onChange={singleSelect ? undefined : () => toggleModifier(mod.id)}
                                    onClick={singleSelect ? () => toggleModifier(mod.id) : undefined}
                                    readOnly={singleSelect}
                                    style={{ width: 18, height: 18, accentColor: "var(--accent)" }}
                                  />
                                </div>
                              </label>
                            ))}
                          </div>
                        </div>
                      )
                    })}
                  </div>
                )}
              </div>
            )}

            {/* Special note */}
            <div>
              <span className="eyebrow" style={{ display: "block", marginBottom: 8 }}>Special note</span>
              <textarea
                value={sheet.note}
                onChange={(e) => setSheet((s) => s ? { ...s, note: e.target.value } : s)}
                placeholder="e.g. No onions, extra spicy…"
                rows={2}
                style={{
                  width: "100%", borderRadius: "var(--rad-lg)", padding: "11px 14px",
                  background: "var(--bg-elev-2)", border: "1px solid var(--line-2)",
                  color: "var(--ink-1)", fontSize: 14, lineHeight: 1.5,
                  resize: "none", outline: "none",
                }}
              />
            </div>

            {/* Sticky Add-to-Order footer */}
            <div style={{
              position: "sticky", bottom: -20,
              marginLeft: -20, marginRight: -20, marginTop: 4,
              padding: "12px 20px",
              paddingBottom: "calc(16px + env(safe-area-inset-bottom))",
              background: "var(--bg-overlay)",
              backdropFilter: "blur(12px)", WebkitBackdropFilter: "blur(12px)",
              borderTop: "1px solid var(--line-1)",
            }}>
              <button
                onClick={handleAddToCart}
                disabled={adding || !requiredGroupsFulfilled}
                className="press"
                style={{
                  width: "100%", height: 54, borderRadius: "var(--rad-md)",
                  display: "flex", alignItems: "center", justifyContent: "space-between",
                  padding: "0 20px",
                  background: adding || !requiredGroupsFulfilled ? "var(--bg-sunken)" : "var(--accent)",
                  color: adding || !requiredGroupsFulfilled ? "var(--ink-3)" : "var(--accent-ink)",
                  fontSize: 15.5, fontWeight: 600, border: "none",
                  boxShadow: adding || !requiredGroupsFulfilled ? "none" : "var(--shadow-2)",
                  transition: "background var(--dur-fast) var(--ease)",
                }}
              >
                <span>{adding ? "Adding…" : "Add to order"}</span>
                <span style={{ fontVariantNumeric: "tabular-nums" }}>
                  {formatCurrency(
                    (parseFloat(String(sheet.item.price)) +
                      sheet.selectedModifiers.reduce((sum, id) => {
                        const mod = sheet.item.modifiers?.find((m) => m.id === id)
                        return sum + (mod ? parseFloat(String(mod.price_delta)) : 0)
                      }, 0)) * sheet.quantity
                  )}
                </span>
              </button>
            </div>
          </div>
        )}
      </BottomSheet>
    </div>
  )
}
