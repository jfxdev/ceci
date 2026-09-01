import { describe, expect, it } from "vitest"

import i18n, { en, ptBR } from "@/lib/i18n"

function keys(value: object, prefix = ""): string[] {
  return Object.entries(value).flatMap(([key, child]) =>
    child && typeof child === "object"
      ? keys(child, `${prefix}${key}.`)
      : [`${prefix}${key}`],
  )
}

describe("i18n resources", () => {
  it("keeps English and Brazilian Portuguese keys in sync", () => {
    expect(keys(ptBR)).toEqual(keys(en))
  })

  it("switches translations without reloading the application", async () => {
    await i18n.changeLanguage("pt-BR")
    expect(i18n.t("auth.signIn")).toBe("Entrar")
    await i18n.changeLanguage("en")
    expect(i18n.t("auth.signIn")).toBe("Sign in")
  })
})
