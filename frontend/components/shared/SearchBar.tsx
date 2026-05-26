"use client"

import { useEffect, useMemo, useRef, useState } from "react"
import { Search, X } from "lucide-react"
import { useMenuStore } from "@/store/menu"
import { debounce } from "@/lib/utils"

interface Props {
  onClose: () => void
}

export function SearchBar({ onClose }: Props) {
  const searchQuery = useMenuStore((s) => s.searchQuery)
  const setSearchQuery = useMenuStore((s) => s.setSearchQuery)
  const [localQuery, setLocalQuery] = useState(searchQuery)
  const inputRef = useRef<HTMLInputElement>(null)

  const debouncedSetQuery = useMemo(
    () => debounce((q: string) => setSearchQuery(q), 150),
    [setSearchQuery]
  )

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  function handleChange(q: string) {
    setLocalQuery(q)
    debouncedSetQuery(q)
  }

  function handleClear() {
    setLocalQuery("")
    setSearchQuery("")
    inputRef.current?.focus()
  }

  return (
    <div
      role="search"
      style={{
        padding: "10px 16px",
        background: "var(--bg-base)",
        borderBottom: "1px solid var(--line-2)",
        display: "flex",
        gap: 10,
        alignItems: "center",
        animation: "sheetUp 0.22s var(--ease-out)",
      }}
    >
      <Search style={{ width: 16, height: 16, color: "var(--ink-3)", flexShrink: 0 }} aria-hidden />
      <input
        ref={inputRef}
        type="search"
        role="searchbox"
        aria-label="Search menu"
        placeholder="Search dishes, ingredients…"
        value={localQuery}
        onChange={(e) => handleChange(e.target.value)}
        style={{
          flex: 1,
          background: "transparent",
          border: "none",
          outline: "none",
          color: "var(--ink-1)",
          fontSize: 15,
        }}
      />
      {localQuery && (
        <button
          onClick={handleClear}
          style={{
            background: "none",
            border: "none",
            color: "var(--ink-3)",
            padding: 4,
            cursor: "pointer",
            display: "flex",
            alignItems: "center",
          }}
          aria-label="Clear search"
        >
          <X style={{ width: 14, height: 14 }} aria-hidden />
        </button>
      )}
      <button
        onClick={onClose}
        style={{
          background: "none",
          border: "none",
          color: "var(--ink-2)",
          fontSize: 13,
          cursor: "pointer",
          whiteSpace: "nowrap",
          padding: "4px 0",
        }}
      >
        Cancel
      </button>
    </div>
  )
}
