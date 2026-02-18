import { computed, ref, watch } from "vue";

// Theme mode type
export type ThemeMode = "auto" | "light" | "dark";
export type ActualTheme = "light" | "dark";

// Storage key name
const THEME_KEY = "gpt-load-theme-mode";

// Get initial theme mode
function getInitialThemeMode(): ThemeMode {
  const stored = localStorage.getItem(THEME_KEY);
  if (stored && ["auto", "light", "dark"].includes(stored)) {
    return stored as ThemeMode;
  }
  return "auto"; // Default to auto mode
}

// Detect system theme preference
function getSystemTheme(): ActualTheme {
  if (window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches) {
    return "dark";
  }
  return "light";
}

// Theme mode (user selection)
export const themeMode = ref<ThemeMode>(getInitialThemeMode());

// System theme (auto-detected)
const systemTheme = ref<ActualTheme>(getSystemTheme());

// Actual theme in use
export const actualTheme = computed<ActualTheme>(() => {
  if (themeMode.value === "auto") {
    return systemTheme.value;
  }
  return themeMode.value as ActualTheme;
});

// Is dark mode
export const isDark = computed(() => actualTheme.value === "dark");

// Switch theme mode
export function setThemeMode(mode: ThemeMode) {
  themeMode.value = mode;
  localStorage.setItem(THEME_KEY, mode);
}

// Toggle theme (for button)
export function toggleTheme() {
  const modes: ThemeMode[] = ["auto", "light", "dark"];
  const currentIndex = modes.indexOf(themeMode.value);
  const nextIndex = (currentIndex + 1) % modes.length;
  setThemeMode(modes[nextIndex]);
}

// Listen for system theme changes
if (window.matchMedia) {
  const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");

  // Update system theme
  const updateSystemTheme = (e: MediaQueryListEvent | MediaQueryList) => {
    systemTheme.value = e.matches ? "dark" : "light";
  };

  // Add listener
  mediaQuery.addEventListener("change", updateSystemTheme);
}

// Update HTML root element class (for CSS variable switching)
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
