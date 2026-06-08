"use client"

import { useRef, useState, useEffect } from "react"
import { Camera, X } from "lucide-react"
import { toast } from "sonner"
import { uploadApi } from "@/lib/api/upload"
import { useStaffStore } from "@/store/staff"

const ALLOWED_TYPES = ["image/jpeg", "image/png", "image/webp"]
const MAX_BYTES = 5 * 1024 * 1024

interface ImageUploadFieldProps {
  currentUrl: string | null | undefined
  onUploaded: (publicUrl: string) => void
  onRemoved?: () => void
  uploadEndpoint: "menu-item" | "restaurant-logo"
  itemId?: number // required when uploadEndpoint === "menu-item"
}

export function ImageUploadField({
  currentUrl,
  onUploaded,
  onRemoved,
  uploadEndpoint,
  itemId,
}: ImageUploadFieldProps) {
  const token = useStaffStore((s) => s.token)
  const inputRef = useRef<HTMLInputElement>(null)
  const abortRef = useRef<AbortController | null>(null)

  const [previewUrl, setPreviewUrl] = useState<string | null>(currentUrl ?? null)
  const [uploading, setUploading] = useState(false)
  const [progress, setProgress] = useState(0)

  useEffect(() => {
    return () => {
      abortRef.current?.abort()
    }
  }, [])

  async function handleFile(file: File) {
    if (!ALLOWED_TYPES.includes(file.type)) {
      toast.error("Only JPG, PNG, or WebP images are allowed")
      return
    }
    if (file.size > MAX_BYTES) {
      toast.error("Image must be under 5 MB")
      return
    }

    const localPreview = URL.createObjectURL(file)
    setPreviewUrl(localPreview)
    setUploading(true)
    setProgress(0)

    const ac = new AbortController()
    abortRef.current = ac

    // Fake progress indication (R2 doesn't support XHR progress on presigned PUT)
    const progressTimer = setInterval(() => {
      setProgress((p) => Math.min(p + 10, 85))
    }, 150)

    try {
      let presign: { upload_url: string; public_url: string }
      if (uploadEndpoint === "menu-item") {
        if (!itemId) throw new Error("itemId is required for menu-item upload")
        presign = await uploadApi.requestMenuItemPresignUrl(itemId, file.type, file.size, token!)
      } else {
        presign = await uploadApi.requestLogoPresignUrl(file.type, file.size, token!)
      }

      await uploadApi.uploadFileToR2(presign.upload_url, file, ac.signal)

      clearInterval(progressTimer)
      setProgress(100)
      setTimeout(() => setUploading(false), 300)
      onUploaded(presign.public_url)
    } catch (err: unknown) {
      clearInterval(progressTimer)
      setUploading(false)
      setPreviewUrl(currentUrl ?? null)
      if (err instanceof Error && err.name === "AbortError") return
      toast.error("Upload failed — please try again")
    } finally {
      URL.revokeObjectURL(localPreview)
    }
  }

  function handleRemove() {
    abortRef.current?.abort()
    setPreviewUrl(null)
    setUploading(false)
    if (inputRef.current) inputRef.current.value = ""
    onRemoved?.()
  }

  if (previewUrl) {
    return (
      <div style={{ position: "relative", width: "100%", borderRadius: 12, overflow: "hidden", background: "var(--bg-elev-2)", border: "1px solid var(--line-2)" }}>
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={previewUrl}
          alt=""
          style={{ width: "100%", height: 160, objectFit: "cover", display: "block" }}
        />
        {uploading && (
          <div style={{
            position: "absolute", bottom: 0, left: 0, right: 0,
            height: 3, background: "rgba(0,0,0,0.2)",
          }}>
            <div style={{
              height: "100%",
              width: `${progress}%`,
              background: "var(--accent)",
              transition: "width 0.15s ease",
            }} />
          </div>
        )}
        {!uploading && (
          <button
            type="button"
            onClick={handleRemove}
            aria-label="Remove image"
            style={{
              position: "absolute", top: 8, right: 8,
              width: 28, height: 28, borderRadius: "50%",
              background: "rgba(0,0,0,0.55)",
              border: "none", cursor: "pointer",
              display: "flex", alignItems: "center", justifyContent: "center",
              color: "#fff",
            }}
          >
            <X size={14} />
          </button>
        )}
      </div>
    )
  }

  return (
    <label
      style={{
        display: "flex", flexDirection: "column", alignItems: "center",
        justifyContent: "center", gap: 8,
        width: "100%", height: 120,
        borderRadius: 12,
        border: "1.5px dashed var(--line-2)",
        background: "var(--bg-elev-1)",
        cursor: "pointer",
        color: "var(--ink-3)",
      }}
    >
      <Camera size={22} strokeWidth={1.5} />
      <span style={{ fontSize: 13, fontWeight: 500 }}>Tap to add a photo</span>
      <span style={{ fontSize: 11, color: "var(--ink-4)" }}>JPG, PNG, WebP · max 5 MB</span>
      <input
        ref={inputRef}
        type="file"
        accept="image/jpeg,image/png,image/webp"
        style={{ display: "none" }}
        onChange={(e) => {
          const file = e.target.files?.[0]
          if (file) handleFile(file)
        }}
      />
    </label>
  )
}
