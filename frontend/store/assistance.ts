import { create } from "zustand"
import type { AssistanceRequest, AssistanceStatus } from "@/types/api"

interface AssistanceState {
  requests: AssistanceRequest[]

  setRequests: (requests: AssistanceRequest[]) => void
  addRequest: (request: AssistanceRequest) => void
  updateStatus: (id: number, status: AssistanceStatus) => void
}

export const useAssistanceStore = create<AssistanceState>((set) => ({
  requests: [],

  setRequests(requests) {
    set({ requests })
  },

  addRequest(request) {
    set((s) => ({
      requests: [request, ...s.requests.filter((r) => r.id !== request.id)],
    }))
  },

  updateStatus(id, status) {
    set((s) => ({
      requests: s.requests.map((r) => (r.id === id ? { ...r, status } : r)),
    }))
  },
}))
