import { render, screen } from "@testing-library/react"
import { describe, expect, it } from "vitest"
import { RoleBadge } from "./role-badge"

describe("RoleBadge", () => {
  it("renders the role text", () => {
    render(<RoleBadge role="owner" />)
    expect(screen.getByText("owner")).toBeInTheDocument()
  })

  it("applies an extra className via props", () => {
    render(<RoleBadge role="viewer" className="extra-class" />)
    expect(screen.getByText("viewer")).toHaveClass("extra-class")
  })
})
