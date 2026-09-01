import { createContext, useContext, useEffect, useState, type ReactNode } from "react"

export const colorThemes = [
  { value: "green", labelKey: "pages.colorThemeGreen", swatches: ["#059669", "#d1fae5", "#a7f3d0"] },
  { value: "blue", labelKey: "pages.colorThemeBlue", swatches: ["#2563eb", "#dbeafe", "#bfdbfe"] },
  { value: "violet", labelKey: "pages.colorThemeViolet", swatches: ["#7c3aed", "#ede9fe", "#ddd6fe"] },
  { value: "orange", labelKey: "pages.colorThemeOrange", swatches: ["#ea580c", "#ffedd5", "#fed7aa"] },
  { value: "rose", labelKey: "pages.colorThemeRose", swatches: ["#e11d48", "#ffe4e6", "#fecdd3"] },
] as const

export type ColorTheme = (typeof colorThemes)[number]["value"]

const DEFAULT_COLOR_THEME: ColorTheme = "green"
const COLOR_THEME_STORAGE_KEY = "ceci-color-theme"

interface ColorThemeContextValue {
  colorTheme: ColorTheme
  setColorTheme: (theme: ColorTheme) => void
}

const ColorThemeContext = createContext<ColorThemeContextValue | null>(null)

function isColorTheme(value: string | null): value is ColorTheme {
  return colorThemes.some((theme) => theme.value === value)
}

function getInitialColorTheme(): ColorTheme {
  if (typeof window === "undefined") return DEFAULT_COLOR_THEME

  const savedTheme = window.localStorage.getItem(COLOR_THEME_STORAGE_KEY)
  return isColorTheme(savedTheme) ? savedTheme : DEFAULT_COLOR_THEME
}

export function ColorThemeProvider({ children }: { children: ReactNode }) {
  const [colorTheme, setColorTheme] = useState<ColorTheme>(getInitialColorTheme)

  useEffect(() => {
    document.documentElement.dataset.colorTheme = colorTheme
    window.localStorage.setItem(COLOR_THEME_STORAGE_KEY, colorTheme)
  }, [colorTheme])

  return <ColorThemeContext.Provider value={{ colorTheme, setColorTheme }}>{children}</ColorThemeContext.Provider>
}

export function useColorTheme() {
  const context = useContext(ColorThemeContext)

  if (!context) {
    throw new Error("useColorTheme must be used within ColorThemeProvider")
  }

  return context
}
