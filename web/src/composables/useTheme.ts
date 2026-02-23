import { computed, ref, watch } from "vue";

export type ThemeMode = "auto" | "light" | "dark";
export type ActualTheme = "light" | "dark";

const THEME_KEY = "gpt-load-theme-mode";

function getInitialThemeMode(): ThemeMode {
  const stored = localStorage.getItem(THEME_KEY);
  if (stored && ["auto", "light", "dark"].includes(stored)) {
    return stored as ThemeMode;
  }
  return "auto";
}

function getSystemTheme(): ActualTheme {
  if (window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches) {
    return "dark";
  }
  return "light";
}

const themeMode = ref<ThemeMode>(getInitialThemeMode());
const systemTheme = ref<ActualTheme>(getSystemTheme());

export const actualTheme = computed<ActualTheme>(() => {
  if (themeMode.value === "auto") {
    return systemTheme.value;
  }
  return themeMode.value as ActualTheme;
});

export const isDark = computed(() => actualTheme.value === "dark");

export function setThemeMode(mode: ThemeMode) {
  themeMode.value = mode;
  localStorage.setItem(THEME_KEY, mode);
}

export function toggleTheme() {
  const modes: ThemeMode[] = ["auto", "light", "dark"];
  const currentIndex = modes.indexOf(themeMode.value);
  const nextIndex = (currentIndex + 1) % modes.length;
  setThemeMode(modes[nextIndex]);
}

export function useTheme() {
  return {
    mode: themeMode,
    actualTheme,
    isDark,
    setThemeMode,
    toggleTheme,
  };
}

if (window.matchMedia) {
  const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");

  const updateSystemTheme = (e: MediaQueryListEvent | MediaQueryList) => {
    systemTheme.value = e.matches ? "dark" : "light";
  };

  mediaQuery.addEventListener("change", updateSystemTheme);
}

watch(
  actualTheme,
  theme => {
    const html = document.documentElement;
    if (theme === "dark") {
      html.classList.add("dark");
      html.classList.remove("light");
    } else {
      html.classList.add("light");
      html.classList.remove("dark");
    }
  },
  { immediate: true }
);
