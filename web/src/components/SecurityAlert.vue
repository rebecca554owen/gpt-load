<script setup lang="ts">
import type { SecurityWarning } from "@/types/models";
import {
  NAlert,
  NButton,
  NCollapse,
  NCollapseItem,
  NList,
  NListItem,
  NSpace,
  NTag,
} from "naive-ui";
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";

const { t } = useI18n();

interface Props {
  warnings: SecurityWarning[];
}

const props = defineProps<Props>();

// Local storage key name
const STORAGE_KEY = "security-alert-dismissed";

// Check if user has permanently dismissed it
const isDismissedPermanently = ref(localStorage.getItem(STORAGE_KEY) === "true");

// Whether closed in current session
const isClosedThisSession = ref(false);

// Whether to show details
const showDetails = ref<string[]>([]);

// Whether to show warning
const shouldShow = computed(() => {
  return props.warnings.length > 0 && !isDismissedPermanently.value && !isClosedThisSession.value;
});

// Get highest severity level
const highestSeverity = computed(() => {
  if (!props.warnings.length) {
    return "low";
  }

  const severityOrder = { high: 3, medium: 2, low: 1 };
  return props.warnings.reduce((highest, warning) => {
    const currentLevel = severityOrder[warning.severity as keyof typeof severityOrder] || 1;
    const highestLevel = severityOrder[highest as keyof typeof severityOrder] || 1;
    return currentLevel > highestLevel ? warning.severity : highest;
  }, "low");
});

// Get warning type mapping (adjusted for gentler colors)
const alertType = computed(() => {
  switch (highestSeverity.value) {
    case "high":
      return "warning"; // 使用橙色而非红色
    case "medium":
      return "info"; // 使用蓝色
    default:
      return "info";
  }
});

// Generate warning summary text
const warningText = computed(() => {
  const count = props.warnings.length;
  const highCount = props.warnings.filter(w => w.severity === "high").length;

  if (highCount > 0) {
    return t("security.warningsWithHigh", { count, highCount });
  } else {
    return t("security.warningsSuggestions", { count });
  }
});

// Get severity label type
const getSeverityTagType = (severity: string) => {
  switch (severity) {
    case "high":
      return "error";
    case "medium":
      return "warning";
    default:
      return "info";
  }
};

// Get severity text
const getSeverityText = (severity: string) => {
  switch (severity) {
    case "high":
      return t("security.important");
    case "medium":
      return t("security.suggestion");
    default:
      return t("security.tip");
  }
};

// Close warning (current session only)
const handleClose = () => {
  isClosedThisSession.value = true;
};

// Don't remind again
const handleDismissPermanently = () => {
  localStorage.setItem(STORAGE_KEY, "true");
  isDismissedPermanently.value = true;
};

// Open security config docs
const openSecurityDocs = () => {
  window.open("https://www.gpt-load.com/docs/configuration/security", "_blank");
};
</script>

<template>
  <n-alert
    v-if="shouldShow"
    :type="alertType"
    :show-icon="false"
    closable
    @close="handleClose"
    style="margin-bottom: 16px"
  >
    <template #header>
      <strong>{{ t("security.configReminder") }}</strong>
    </template>

    <div>
      <div style="margin-bottom: 16px; font-size: 14px">
        {{ warningText }}
      </div>

      <!-- Issue details list -->
      <n-collapse v-model:expanded-names="showDetails" style="margin-bottom: 12px">
        <n-collapse-item name="details" :title="t('security.viewDetails')">
          <n-list style="padding-top: 8px; margin-left: 0">
            <n-list-item
              v-for="(warning, index) in warnings"
              :key="index"
              style="padding: 12px 16px; border-bottom: 1px solid var(--border-color)"
            >
              <template #prefix>
                <n-tag
                  :type="getSeverityTagType(warning.severity)"
                  size="small"
                  style="margin-right: 12px; min-width: 40px; text-align: center"
                >
                  {{ getSeverityText(warning.severity) }}
                </n-tag>
              </template>

              <div style="flex: 1">
                <div
                  style="
                    font-weight: 500;
                    color: var(--text-primary);
                    margin-bottom: 6px;
                    font-size: 14px;
                  "
                >
                  {{ warning.message }}
                </div>
                <div style="font-size: 12px; color: var(--text-secondary); line-height: 1.4">
                  {{ warning.suggestion }}
                </div>
              </div>
            </n-list-item>
          </n-list>
        </n-collapse-item>
      </n-collapse>

      <n-space size="small">
        <n-button
          size="small"
          type="primary"
          @click="openSecurityDocs"
          class="security-primary-btn"
        >
          {{ t("security.configDocs") }}
        </n-button>

        <n-button
          size="small"
          secondary
          @click="handleDismissPermanently"
          class="security-secondary-btn"
        >
          {{ t("security.dontRemind") }}
        </n-button>
      </n-space>
    </div>
  </n-alert>
</template>

<style scoped>
/* Security alert button style optimization */
.security-primary-btn {
  font-weight: 600;
}

.security-secondary-btn {
  font-weight: 500;
}

/* Dark mode button optimization */
:root.dark .security-primary-btn {
  background: var(--primary-color) !important;
  color: white !important;
  border: 1px solid var(--primary-color) !important;
}

:root.dark .security-primary-btn:hover {
  background: var(--primary-color-hover) !important;
  border-color: var(--primary-color-hover) !important;
}

:root.dark .security-secondary-btn {
  background: rgba(255, 255, 255, 0.1) !important;
  color: var(--text-primary) !important;
  border: 1px solid rgba(255, 255, 255, 0.2) !important;
}

:root.dark .security-secondary-btn:hover {
  background: rgba(255, 255, 255, 0.15) !important;
  border-color: rgba(255, 255, 255, 0.3) !important;
}
</style>
