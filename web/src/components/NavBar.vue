<script setup lang="ts">
import AppIcon from "@/components/icons/AppIcon.vue";
import { useAuthService } from "@/composables/useAuth";
import { type MenuOption } from "naive-ui";
import { computed, h, watch } from "vue";
import { RouterLink, useRoute } from "vue-router";
import { useI18n } from "vue-i18n";

const { t } = useI18n();
const { checkLogin } = useAuthService();
const isLoggedIn = computed(() => checkLogin());

interface Props {
  mode?: "horizontal" | "vertical";
}
const props = withDefaults(defineProps<Props>(), {
  mode: "horizontal",
});

const emit = defineEmits(["close"]);

const menuOptions = computed<MenuOption[]>(() => {
  const options: MenuOption[] = [renderMenuItem("dashboard", t("nav.dashboard"), "dashboard")];

  // Admin menu items only shown when logged in
  if (isLoggedIn.value) {
    options.push(
      renderMenuItem("keys", t("nav.keys"), "keys"),
      renderMenuItem("logs", t("nav.logs"), "logs"),
      renderMenuItem("settings", t("nav.settings"), "settings")
    );
  }

  return options;
});

const route = useRoute();
const activeMenu = computed(() => route.name);

watch(activeMenu, () => {
  if (props.mode === "vertical") {
    emit("close");
  }
});

function renderMenuItem(key: string, label: string, iconName: string): MenuOption {
  return {
    label: () =>
      h(
        RouterLink,
        {
          to: {
            name: key,
          },
          class: "nav-menu-item",
        },
        {
          default: () => [
            h(AppIcon, { name: iconName, size: 18, "aria-hidden": "true" }),
            h("span", { class: "nav-item-text" }, label),
          ],
        }
      ),
    key,
  };
}
</script>

<template>
  <div>
    <n-menu :mode="mode" :options="menuOptions" :value="activeMenu" class="modern-menu" />
  </div>
</template>

<style scoped>
:deep(.nav-menu-item) {
  display: flex;
  align-items: center;
  gap: 10px;
  text-decoration: none;
  color: inherit;
  padding: 8px 12px;
  border-radius: var(--radius-md);
  transition: all var(--transition-base);
  font-weight: 500;
}

:deep(.n-menu-item) {
  border-radius: var(--radius-md);
}

:deep(.n-menu--vertical .n-menu-item-content) {
  justify-content: center;
}

:deep(.n-menu--vertical .n-menu-item) {
  margin: 4px 8px;
}

:deep(.n-menu-item:hover) {
  background: var(--hover-bg);
  transform: translateY(-1px);
  border-radius: var(--radius-md);
}

:deep(.n-menu-item--selected) {
  background: var(--primary-gradient);
  color: var(--text-inverse);
  font-weight: 600;
  box-shadow: var(--shadow-md);
  border-radius: var(--radius-md);
}

:deep(.n-menu-item--selected:hover) {
  background: var(--primary-color-hover);
  transform: translateY(-1px);
}

:deep(.n-menu-item--selected svg) {
  color: var(--text-inverse);
}
</style>
