import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"
import { DataTable, type DataTableColumn } from "./data-table"

interface Row {
  id: string
  name: string
}

const columns: DataTableColumn<Row>[] = [{ key: "name", header: "Name", render: (row) => row.name }]

describe("DataTable", () => {
  it("renders rows using the provided render function", () => {
    render(<DataTable columns={columns} rows={[{ id: "1", name: "Alpha" }]} rowKey={(r) => r.id} />)
    expect(screen.getByText("Alpha")).toBeInTheDocument()
  })

  it("shows the empty message when there are no rows", () => {
    render(<DataTable columns={columns} rows={[]} rowKey={(r) => r.id} emptyMessage="Nothing here" />)
    expect(screen.getByText("Nothing here")).toBeInTheDocument()
  })

  it("invokes onRowClick with the clicked row", async () => {
    const onRowClick = vi.fn()
    const user = userEvent.setup()
    render(
      <DataTable columns={columns} rows={[{ id: "1", name: "Alpha" }]} rowKey={(r) => r.id} onRowClick={onRowClick} />,
    )
    await user.click(screen.getByText("Alpha"))
    expect(onRowClick).toHaveBeenCalledWith({ id: "1", name: "Alpha" })
  })
})
