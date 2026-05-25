import { Skeleton } from "@/components/ui/skeleton"

export function MenuSkeleton() {
  return (
    <div className="px-5 py-5 space-y-0">
      {/* Category pills */}
      <div className="flex gap-1.5 pb-4 overflow-hidden">
        {Array.from({ length: 4 }).map((_, i) => (
          <Skeleton key={i} className="h-9 w-20 rounded-full flex-shrink-0" />
        ))}
      </div>
      {/* Category label */}
      <Skeleton className="h-3 w-24 mb-3" />
      {/* Divider-list items */}
      {Array.from({ length: 6 }).map((_, i) => (
        <div key={i} className="py-4 border-b border-[var(--line-1)] flex justify-between items-start gap-4">
          <div className="flex-1 space-y-2">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-3 w-full" />
            <Skeleton className="h-3 w-16 mt-1" />
          </div>
          <Skeleton className="size-16 rounded-xl flex-shrink-0" />
        </div>
      ))}
    </div>
  )
}

export function CartSkeleton() {
  return (
    <div className="px-5 py-6 space-y-0">
      {/* Items */}
      {Array.from({ length: 3 }).map((_, i) => (
        <div key={i} className="py-4 border-b border-[var(--line-1)] flex gap-3 items-center">
          <Skeleton className="size-8 rounded-full flex-shrink-0" />
          <div className="flex-1 space-y-1.5">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-3 w-1/3" />
          </div>
          <Skeleton className="h-4 w-16" />
        </div>
      ))}
      {/* Summary card */}
      <div className="mt-5 rounded-[var(--rad-lg)] border border-[var(--line-2)] p-4 space-y-3">
        <div className="flex justify-between">
          <Skeleton className="h-4 w-20" />
          <Skeleton className="h-4 w-24" />
        </div>
        <div className="flex justify-between">
          <Skeleton className="h-3 w-16" />
          <Skeleton className="h-3 w-20" />
        </div>
      </div>
      <Skeleton className="h-12 w-full rounded-xl mt-4" />
    </div>
  )
}

export function OrderSkeleton() {
  return (
    <div className="px-4 py-6 space-y-4">
      {Array.from({ length: 2 }).map((_, i) => (
        <div key={i} className="rounded-xl p-4 border border-[var(--line-2)] space-y-3">
          <div className="flex justify-between items-center">
            <Skeleton className="h-4 w-32" />
            <Skeleton className="h-6 w-20 rounded-full" />
          </div>
          <div className="space-y-2">
            <Skeleton className="h-3 w-full" />
            <Skeleton className="h-3 w-3/4" />
          </div>
          <div className="flex gap-1">
            {Array.from({ length: 5 }).map((_, j) => (
              <Skeleton key={j} className="h-1.5 flex-1 rounded-full" />
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}
