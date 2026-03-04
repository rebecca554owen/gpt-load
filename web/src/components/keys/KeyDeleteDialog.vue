<script setup lang="ts">
import { keysApi } from "@/api/keys";
import { useAppStateStore } from "@/stores/appState";
import { Close } from "@vicons/ionicons5";
import { NButton, NCard, NInput, NModal } from "naive-ui";
import { ref, watch } from "vue";
import { useI18n } from "vue-i18n";

interface Props {
  show: boolean;
  groupId: number;
  groupName?: string;
}

interface Emits {
  (e: "update:show", value: boolean): void;
  (e: "success"): void;
}

const props = defineProps<Props>();

const emit = defineEmits<Emits>();

const { t } = useI18n();

const loading = ref(false);
const keysText = ref("");

// 监听弹窗显示状态
watch(
  () => props.show,
  show => {
    if (show) {
      resetForm();
    }
  }
);

// 重置表单
function resetForm() {
  keysText.value = "";
}

// 关闭弹窗
function handleClose() {
  emit("update:show", false);
}

// 提交表单
async function handleSubmit() {
  if (loading.value || !keysText.value.trim()) {
    return;
  }

  try {
    loading.value = true;
    const appState = useAppStateStore();

    await keysApi.deleteKeysAsync(props.groupId, keysText.value);
    resetForm();

    handleClose();
    window.$message.success(t("keys.deleteTaskStarted"));
    appState.triggerTaskPolling();
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <n-modal :show="show" @update:show="handleClose" class="form-modal">
    <n-card
      style="width: 800px"
      :title="t('keys.deleteKeysFromGroup', { group: groupName || t('keys.currentGroup') })"
      :bordered="false"
      size="huge"
      role="dialog"
      aria-modal="true"
    >
      <template #header-extra>
        <n-button quaternary circle @click="handleClose" class="modal-close">
          <template #icon>
            <n-icon :component="Close" />
          </template>
        </n-button>
      </template>

      <n-input
        v-model:value="keysText"
        type="textarea"
        :placeholder="t('keys.enterKeysToDeletePlaceholder')"
        :rows="8"
        style="margin-top: 20px"
      />

      <template #footer>
        <div class="modal-footer">
          <n-button @click="handleClose" class="btn-cancel">{{ t("common.cancel") }}</n-button>
          <n-button
            @click="handleSubmit"
            :loading="loading"
            :disabled="!keysText"
            class="btn-delete"
          >
            {{ t("common.delete") }}
          </n-button>
        </div>
      </template>
    </n-card>
  </n-modal>
</template>

<style scoped>
.form-modal {
  --n-color: rgba(255, 255, 255, 0.95);
}

:deep(.n-input) {
  --n-border-radius: 6px;
}

:deep(.n-card-header) {
  border-bottom: 1px solid rgba(239, 239, 245, 0.8);
  padding: 10px 20px;
}

:deep(.n-card__content) {
  max-height: calc(100vh - 68px - 61px - 50px);
  overflow-y: auto;
}
</style>
