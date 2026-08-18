import { render, screen } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"

const mocks = vi.hoisted(() => ({ get: vi.fn() }))

vi.mock("@/lib/api", () => ({ api: { get: mocks.get } }))
vi.mock("@/lib/auth", () => ({
  useAuth: () => ({ user: { id: "u1", email: "admin@example.com", name: "Admin", isAdmin: true } }),
}))

import { MaintenanceProvider, useMaintenance } from "@/lib/maintenance"

function Status() {
  const { status } = useMaintenance()
  return <span>{status.enabled ? status.message : "inactive"}</span>
}

describe("MaintenanceProvider", () => {
  it("loads the custom maintenance message for authenticated users", async () => {
    mocks.get.mockResolvedValue({ enabled: true, message: "Database migration", startedAt: "2026-08-13T12:00:00Z" })
    render(<MaintenanceProvider><Status /></MaintenanceProvider>)
    expect(await screen.findByText("Database migration")).toBeInTheDocument()
    expect(mocks.get).toHaveBeenCalledWith("/maintenance-status")
  })
})
