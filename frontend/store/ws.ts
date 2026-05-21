import { create } from "zustand"

type WSStatus = "connected" | "reconnecting" | "disconnected"

interface WsState {
  status: WSStatus
  attempt: number

  setStatus: (status: WSStatus, attempt?: number) => void
}

export const useWsStore = create<WsState>((set) => ({
  status: "disconnected",
  attempt: 0,

  setStatus(status, attempt = 0) {
    set({ status, attempt })
  },
}))
