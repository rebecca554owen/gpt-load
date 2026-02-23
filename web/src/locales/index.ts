import axios from "axios";
import { createI18n } from "vue-i18n";
import enUS from "./en-US";
import jaJP from "./ja-JP";
import zhCN from "./zh-CN";

// Supported languages list
export const SUPPORTED_LOCALES = [
  { key: "zh-CN", label: "中文" },
  { key: "en-US", label: "English" },
  { key: "ja-JP", label: "日本語" },
] as const;

export type Locale = (typeof SUPPORTED_LOCALES)[number]["key"];

// Get default locale
function getDefaultLocale(): Locale {
  // 1. Prefer saved locale from localStorage
  const savedLocale = localStorage.getItem("locale");
  if (savedLocale && SUPPORTED_LOCALES.some(l => l.key === savedLocale)) {
    return savedLocale as Locale;
  }

  // 2. Auto-detect browser language
  const browserLang = navigator.language;

  // Exact match
  if (SUPPORTED_LOCALES.some(l => l.key === browserLang)) {
    return browserLang as Locale;
  }

  // Fuzzy match (e.g., zh matches zh-CN)
  const shortLang = browserLang.split("-")[0];
  const matched = SUPPORTED_LOCALES.find(l => l.key.startsWith(shortLang));
  if (matched) {
    return matched.key;
  }

  // 3. Default to Chinese
  return "zh-CN";
}

// Create i18n instance
const defaultLocale = getDefaultLocale();
const i18n = createI18n({
  legacy: false,
  locale: defaultLocale,
  fallbackLocale: "zh-CN",
  messages: {
    "zh-CN": zhCN,
    "en-US": enUS,
    "ja-JP": jaJP,
  },
});

// Set axios default language on initialization
if (axios.defaults.headers) {
  axios.defaults.headers.common["Accept-Language"] = defaultLocale;
}

// Helper function to switch language
export function setLocale(locale: Locale) {
  // Save to localStorage
  localStorage.setItem("locale", locale);

  // Update axios default headers
  if (axios.defaults.headers) {
    axios.defaults.headers.common["Accept-Language"] = locale;
  }

  // Reload page to ensure all content (including backend data) uses new language
  window.location.reload();
}

// Get current locale
export function getLocale(): Locale {
  return i18n.global.locale.value as string as Locale;
}

// Get current locale label
export function getCurrentLocaleLabel(): string {
  const current = getLocale();
  return SUPPORTED_LOCALES.find(l => l.key === current)?.label || "中文";
}

export default i18n;
