import { render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { beforeEach, describe, expect, it, vi } from "vitest"

const mocks = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn(), delete: vi.fn() }))

vi.mock("@/lib/api", () => ({
  api: mocks,
  ApiError: class ApiError extends Error {},
}))

vi.mock("@/lib/environment", () => ({
  useEnvironment: () => ({
    envKey: "production",
    envPath: (suffix: string) => `/projects/project-1/environments/production/${suffix}`,
  }),
}))

import { parameterPath, ParametersBrowserPage } from "./parameters-browser"

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={["/projects/project-1/parameters"]}>
        <Routes>
          <Route path="/projects/:projectId/parameters" element={<ParametersBrowserPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe("ParametersBrowserPage", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.get.mockResolvedValue([{ key: "service/db/host", value: "localhost", version: 2 }])
    mocks.put.mockResolvedValue(undefined)
    mocks.delete.mockResolvedValue(undefined)
  })

  it("opens a prefilled, key-locked form and saves parameter edits", async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole("button", { name: "Edit" }))

    expect(screen.getByRole("heading", { name: 'Edit "service/db/host"' })).toBeInTheDocument()
    expect(screen.getByLabelText("Key")).toHaveValue("service/db/host")
    expect(screen.getByLabelText("Key")).toBeDisabled()
    expect(screen.getByLabelText("Value")).toHaveValue("localhost")

    await user.clear(screen.getByLabelText("Value"))
    await user.type(screen.getByLabelText("Value"), "database.internal")
    await user.click(screen.getByRole("button", { name: "Save changes" }))

    await waitFor(() => {
      expect(mocks.put).toHaveBeenCalledWith(
        "/projects/project-1/environments/production/parameters/value/service/db/host",
        { value: "database.internal" },
      )
    })
  })

  it("asks for confirmation before deleting a parameter", async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole("button", { name: "Delete" }))
    const dialog = await screen.findByRole("dialog")
    expect(within(dialog).getByText('Delete "service/db/host"?')).toBeInTheDocument()

    await user.click(within(dialog).getByRole("button", { name: "Delete" }))

    await waitFor(() => {
      expect(mocks.delete).toHaveBeenCalledWith(
        "/projects/project-1/environments/production/parameters/value/service/db/host",
      )
    })
  })

  it("encodes unsafe key characters without flattening its hierarchy", () => {
    expect(parameterPath("service/db host?blue#1")).toBe("service/db%20host%3Fblue%231")
  })
})
