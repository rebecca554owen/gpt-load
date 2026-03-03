<script setup lang="ts">
import { useAppStateStore } from "@/stores/appState";
import { actualTheme } from "@/composables/useTheme";
import { getLocale } from "@/locales";
import {
  darkTheme,
  NConfigProvider,
  NDialogProvider,
  NLoadingBarProvider,
  NMessageProvider,
  useLoadingBar,
  useMessage,
  type GlobalTheme,
  type GlobalThemeOverrides,
  zhCN,
  enUS,
  jaJP,
  dateZhCN,
  dateEnUS,
  dateJaJP,
} from "naive-ui";
import { computed, defineComponent, watch } from "vue";

const getCssVar = (varName: string): string => {
  if (typeof window === "undefined") {
    return "";
  }
  return getComputedStyle(document.documentElement).getPropertyValue(varName).trim() || "";
};

const rgba = (hexOrRgba: string, alpha: number): string => {
  const hex = hexOrRgba.replace("#", "");
  if (hex.length === 6) {
    const r = Number.parseInt(hex.slice(0, 2), 16);
    const g = Number.parseInt(hex.slice(2, 4), 16);
    const b = Number.parseInt(hex.slice(4, 6), 16);
    return `rgba(${r}, ${g}, ${b}, ${alpha})`;
  }
  return hexOrRgba;
};

const themeOverrides = computed<GlobalThemeOverrides>(() => {
  const primaryColor = getCssVar("--primary-color");
  const primaryColorHover = getCssVar("--primary-color-hover");
  const primaryColorPressed = getCssVar("--primary-color-pressed");
  const primaryColorSuppl = getCssVar("--primary-color-suppl");
  const errorColor = getCssVar("--error-color");

  const baseOverrides: GlobalThemeOverrides = {
    common: {
      primaryColor,
      primaryColorHover,
      primaryColorPressed,
      primaryColorSuppl,
      borderRadius: "10px",
      borderRadiusSmall: "6px",
      fontFamily: "'DM Sans', system-ui, sans-serif",
    },
    Card: {
      paddingMedium: "20px",
    },
    Button: {
      fontWeight: "600",
      heightMedium: "38px",
      heightLarge: "44px",
    },
    Input: {
      heightMedium: "38px",
      heightLarge: "44px",
    },
    Menu: {
      itemHeight: "40px",
    },
    LoadingBar: {
      colorLoading: primaryColor,
      colorError: errorColor,
      height: "3px",
    },
  };

  if (actualTheme.value === "dark") {
    const bodyColor = getCssVar("--bg-primary");
    const cardColor = getCssVar("--card-bg-solid");
    const inputColor = getCssVar("--bg-tertiary");
    const textColorBase = getCssVar("--text-primary");
    const textColor2 = getCssVar("--text-secondary");
    const textColor3 = getCssVar("--text-tertiary");
    const borderColor = getCssVar("--border-color");
    const dividerColor = getCssVar("--border-color-light");

    return {
      ...baseOverrides,
      common: {
        ...baseOverrides.common,
        bodyColor,
        cardColor,
        modalColor: cardColor,
        popoverColor: cardColor,
        tableColor: cardColor,
        inputColor,
        actionColor: inputColor,
        textColorBase,
        textColor1: textColorBase,
        textColor2,
        textColor3,
        borderColor,
        dividerColor,
      },
      Card: {
        ...baseOverrides.Card,
        color: cardColor,
        textColor: textColorBase,
        borderColor,
      },
      Input: {
        ...baseOverrides.Input,
        color: inputColor,
        textColor: textColorBase,
        colorFocus: inputColor,
        borderHover: rgba(primaryColor, 0.4),
        borderFocus: rgba(primaryColor, 0.6),
        placeholderColor: textColor3,
      },
      Select: {
        peers: {
          InternalSelection: {
            textColor: textColorBase,
            color: inputColor,
            placeholderColor: textColor3,
          },
        },
      },
      DataTable: {
        tdColor: cardColor,
        thColor: inputColor,
        thTextColor: textColorBase,
        tdTextColor: textColorBase,
        borderColor,
      },
      Tag: {
        textColor: textColorBase,
      },
      Pagination: {
        itemTextColor: textColor2,
        itemTextColorActive: textColorBase,
        itemColor: inputColor,
        itemColorActive: borderColor,
      },
      DatePicker: {
        itemTextColor: textColorBase,
        itemColorActive: inputColor,
        panelColor: cardColor,
      },
      Message: {
        color: inputColor,
        textColor: textColorBase,
        iconColor: textColorBase,
        borderRadius: "8px",
        colorInfo: inputColor,
        colorSuccess: inputColor,
        colorWarning: inputColor,
        colorError: inputColor,
        colorLoading: inputColor,
      },
      LoadingBar: {
        ...baseOverrides.LoadingBar,
      },
      Notification: {
        color: inputColor,
        textColor: textColorBase,
        titleTextColor: textColorBase,
        descriptionTextColor: textColor2,
        borderRadius: "8px",
      },
    };
  }

  return baseOverrides;
});

// 根据当前主题动态返回主题对象
const theme = computed<GlobalTheme | undefined>(() => {
  return actualTheme.value === "dark" ? darkTheme : undefined;
});

// 根据当前语言返回对应的语言配置
const locale = computed(() => {
  const currentLocale = getLocale();
  switch (currentLocale) {
    case "zh-CN":
      return zhCN;
    case "en-US":
      return enUS;
    case "ja-JP":
      return jaJP;
    default:
      return zhCN;
  }
});

// 根据当前语言返回对应的日期语言配置
const dateLocale = computed(() => {
  const currentLocale = getLocale();
  switch (currentLocale) {
    case "zh-CN":
      return dateZhCN;
    case "en-US":
      return dateEnUS;
    case "ja-JP":
      return dateJaJP;
    default:
      return dateZhCN;
  }
});

function useGlobalMessage() {
  window.$message = useMessage();
}

const LoadingBar = defineComponent({
  setup() {
    const loadingBar = useLoadingBar();
    const appState = useAppStateStore();
    watch(
      () => appState.loading,
      loading => {
        if (loading) {
          loadingBar.start();
        } else {
          loadingBar.finish();
        }
      }
    );
    return () => null;
  },
});

const Message = defineComponent({
  setup() {
    useGlobalMessage();
    return () => null;
  },
});
</script>

<template>
  <n-config-provider
    :theme="theme"
    :theme-overrides="themeOverrides"
    :locale="locale"
    :date-locale="dateLocale"
  >
    <n-loading-bar-provider>
      <n-message-provider placement="top-right">
        <n-dialog-provider>
          <slot />
          <loading-bar />
          <message />
        </n-dialog-provider>
      </n-message-provider>
    </n-loading-bar-provider>
  </n-config-provider>
</template>
