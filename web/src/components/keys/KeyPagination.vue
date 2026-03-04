<script setup lang="ts">
import { PAGINATION } from "@/constants/chart";
import { NButton, NSelect } from "naive-ui";
import { useI18n } from "vue-i18n";

type PageSize = 12 | 24 | 60 | 120;

interface Props {
  currentPage: number;
  totalPages: number;
  total: number;
  pageSize: PageSize;
}

const emit = defineEmits<{
  "update:current-page": [value: number];
  "update:page-size": [value: PageSize];
}>();

const { t } = useI18n();

const pageSizeOptions = PAGINATION.keyPageSizes.map(size => ({
  label: t("keys.recordsPerPage", { count: size }),
  value: size,
}));

const props = defineProps<Props>();

function handlePrevPage() {
  emit("update:current-page", props.currentPage - 1);
}

function handleNextPage() {
  emit("update:current-page", props.currentPage + 1);
}

function handlePageSizeChange(value: PageSize) {
  emit("update:page-size", value);
}
</script>

<template>
  <div class="pagination-container">
    <div class="pagination-info">
      <span>{{ t("keys.totalRecords", { total }) }}</span>
      <n-select
        :value="pageSize"
        :options="pageSizeOptions"
        size="small"
        style="width: 100px; margin-left: 12px"
        @update:value="handlePageSizeChange"
      />
    </div>
    <div class="pagination-controls">
      <n-button size="small" :disabled="currentPage <= 1" @click="handlePrevPage">
        {{ t("common.previousPage") }}
      </n-button>
      <span class="page-info">
        {{ t("keys.pageInfo", { current: currentPage, total: totalPages }) }}
      </span>
      <n-button size="small" :disabled="currentPage >= totalPages" @click="handleNextPage">
        {{ t("common.nextPage") }}
      </n-button>
    </div>
  </div>
</template>

<style scoped>
.pagination-container {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: var(--card-bg-solid);
  border-top: 1px solid var(--border-color);
  flex-shrink: 0;
  border-radius: 0 0 8px 8px;
}

.pagination-info {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 12px;
  color: var(--text-secondary);
}

.pagination-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

.page-info {
  font-size: 12px;
  color: var(--text-secondary);
}
</style>
