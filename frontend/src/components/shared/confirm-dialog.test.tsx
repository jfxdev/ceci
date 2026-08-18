import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"
import { ConfirmDialog } from "./confirm-dialog"

describe("ConfirmDialog", () => {
  it("renders title and description when open", () => {
    render(
      <ConfirmDialog
        open
        onOpenChange={() => {}}
        title="Delete flag"
        description="This cannot be undone."
        onConfirm={() => {}}
      />,
    )
    expect(screen.getByText("Delete flag")).toBeInTheDocument()
    expect(screen.getByText("This cannot be undone.")).toBeInTheDocument()
  })

  it("calls onConfirm when confirm button is clicked", async () => {
    const onConfirm = vi.fn()
    const user = userEvent.setup()
    render(<ConfirmDialog open onOpenChange={() => {}} title="Delete" onConfirm={onConfirm} confirmLabel="Delete" />)

    await user.click(screen.getByRole("button", { name: "Delete" }))
    expect(onConfirm).toHaveBeenCalledOnce()
  })

  it("calls onOpenChange(false) when cancel is clicked", async () => {
    const onOpenChange = vi.fn()
    const user = userEvent.setup()
    render(<ConfirmDialog open onOpenChange={onOpenChange} title="Delete" onConfirm={() => {}} />)

    await user.click(screen.getByRole("button", { name: "Cancel" }))
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })
})
