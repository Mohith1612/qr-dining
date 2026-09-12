import type { Page } from "@playwright/test"

export async function attemptTicketUpgrade(
  page: Page,
  apiURL: string,
  ticket: string
): Promise<"opened" | "rejected"> {
  const wsURL = apiURL.replace(/^http/, "ws")

  return page.evaluate(
    ({ wsURL, ticket }) =>
      new Promise<"opened" | "rejected">((resolve, reject) => {
        const socket = new WebSocket(`${wsURL}/ws?ticket=${encodeURIComponent(ticket)}`)
        let settled = false
        const finish = (result: "opened" | "rejected") => {
          if (settled) return
          settled = true
          window.clearTimeout(timeout)
          resolve(result)
        }
        const timeout = window.setTimeout(() => {
          socket.close()
          reject(new Error("WebSocket upgrade did not open or reject within 5 seconds"))
        }, 5_000)

        socket.addEventListener("open", () => {
          socket.close(1000, "e2e ticket check complete")
          finish("opened")
        })
        socket.addEventListener("error", () => finish("rejected"))
        socket.addEventListener("close", () => finish("rejected"))
      }),
    { wsURL, ticket }
  )
}
