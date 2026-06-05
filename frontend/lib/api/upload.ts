import { api } from "./client"

interface PresignResponse {
  upload_url: string
  public_url: string
  expires_in: number
}

export const uploadApi = {
  requestMenuItemPresignUrl(
    itemId: number,
    contentType: string,
    fileSize: number,
    staffToken: string
  ): Promise<PresignResponse> {
    return api.post<PresignResponse>(
      "/upload/menu-item-image",
      { item_id: itemId, content_type: contentType, file_size: fileSize },
      { staffToken }
    )
  },

  requestLogoPresignUrl(
    contentType: string,
    fileSize: number,
    staffToken: string
  ): Promise<PresignResponse> {
    return api.post<PresignResponse>(
      "/upload/restaurant-logo",
      { content_type: contentType, file_size: fileSize },
      { staffToken }
    )
  },

  async uploadFileToR2(uploadUrl: string, file: File, signal?: AbortSignal): Promise<void> {
    const res = await fetch(uploadUrl, {
      method: "PUT",
      headers: { "Content-Type": file.type },
      body: file,
      signal,
    })
    if (!res.ok) {
      throw new Error(`R2 upload failed: ${res.status} ${res.statusText}`)
    }
  },
}
