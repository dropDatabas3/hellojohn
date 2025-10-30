// i18n utilities

import { es } from "./es"
import { en } from "./en"
import { useUIStore } from "../ui-store"

export const translations = { es, en }

export type Locale = "es" | "en"

export function getTranslations(locale: Locale) {
  return translations[locale] || translations.es
}

export function useI18n() {
  const locale = useUIStore((state) => state.locale)
  const t = (key: string, params?: Record<string, any>) => {
    const keys = key.split(".")
    let value: any = translations[locale]

    for (const k of keys) {
      value = value?.[k]
    }

    if (typeof value === "string" && params) {
      return value.replace(/\{\{(\w+)\}\}/g, (_, key) => params[key] || "")
    }

    return value || key
  }

  return { t, locale }
}
