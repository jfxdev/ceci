import { fireEvent, render, screen } from "@testing-library/react"
import { afterEach, describe, expect, it } from "vitest"

import { ColorThemeProvider, useColorTheme } from "@/lib/color-theme"

function ThemeControl() {
  const { colorTheme, setColorTheme } = useColorTheme()

  return (
    <>
      <span>{colorTheme}</span>
      <button onClick={() => setColorTheme("blue")}>Use blue</button>
    </>
  )
}

afterEach(() => {
  window.localStorage.clear()
  delete document.documentElement.dataset.colorTheme
})

describe("ColorThemeProvider", () => {
  it("uses green by default and applies it to the document", async () => {
    render(<ColorThemeProvider><ThemeControl /></ColorThemeProvider>)

    expect(await screen.findByText("green")).toBeInTheDocument()
    expect(document.documentElement.dataset.colorTheme).toBe("green")
  })

  it("persists the selected color theme", async () => {
    render(<ColorThemeProvider><ThemeControl /></ColorThemeProvider>)

    fireEvent.click(screen.getByRole("button", { name: "Use blue" }))

    expect(await screen.findByText("blue")).toBeInTheDocument()
    expect(document.documentElement.dataset.colorTheme).toBe("blue")
    expect(window.localStorage.getItem("ceci-color-theme")).toBe("blue")
  })
})
