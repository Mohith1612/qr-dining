import { UtensilsCrossed } from "lucide-react"

export function SessionEndedScreen() {
  return (
    <div
      className="min-h-svh flex flex-col items-center justify-center px-6 text-center gap-6"
      style={{ backgroundColor: "var(--color-bg)", color: "var(--color-text)" }}
    >
      <div
        className="size-16 rounded-full flex items-center justify-center"
        style={{ backgroundColor: "var(--color-surface)", border: "1px solid var(--color-border)" }}
      >
        <UtensilsCrossed className="size-8" style={{ color: "var(--color-text-muted)" }} aria-hidden />
      </div>
      <div className="space-y-2">
        <h1 className="text-xl font-semibold">Your session has ended</h1>
        <p className="text-sm max-w-xs" style={{ color: "var(--color-text-muted)" }}>
          Thank you for dining with us. We hope to see you again soon.
        </p>
      </div>
    </div>
  )
}
