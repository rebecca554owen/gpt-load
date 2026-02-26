<script setup lang="ts">
import { useAuthService } from "@/composables/useAuth";
import { LogInOutline, LogOutOutline } from "@vicons/ionicons5";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { computed } from "vue";

const { t } = useI18n();

const router = useRouter();
const { checkLogin, logout } = useAuthService();
const isLoggedIn = computed(() => checkLogin());

const handleClick = () => {
  if (isLoggedIn.value) {
    logout();
    router.replace("/");
  } else {
    router.push("/login");
  }
};
</script>

<template>
  <n-button quaternary round class="logout-button" @click="handleClick">
    <template #icon>
      <n-icon :component="isLoggedIn ? LogOutOutline : LogInOutline" />
    </template>
    {{ isLoggedIn ? t("nav.logout") : t("nav.login") }}
  </n-button>
</template>

<style scoped>
.logout-button {
  color: var(--text-secondary);
  background: var(--card-bg);
  backdrop-filter: blur(8px);
  border: 1px solid var(--border-color-light);
  transition: all 0.2s ease;
  font-weight: 500;
  letter-spacing: 0.2px;
}

.logout-button:hover {
  color: var(--logout-hover-color);
  background: var(--logout-hover-bg);
  border-color: var(--logout-hover-border);
  transform: translateY(-1px);
  box-shadow: var(--shadow-md);
}

:deep(.n-button__content) {
  gap: 6px;
}
</style>
