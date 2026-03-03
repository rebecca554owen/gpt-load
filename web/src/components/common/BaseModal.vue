<script setup lang="ts">
import { Close } from "@vicons/ionicons5";
import { NButton, NIcon, NModal } from "naive-ui";
import { computed } from "vue";
import { useI18n } from "vue-i18n";

interface Props {
  show: boolean;
  title?: string;
  confirmText?: string;
  cancelText?: string;
  loading?: boolean;
  width?: string | number;
  maxWidth?: string;
  height?: string | number;
  maxHeight?: string;
  modalClass?: string;
  cardClass?: string;
  showCloseButton?: boolean;
  closable?: boolean;
  maskClosable?: boolean;
  preventCloseOnLoading?: boolean;
  footer?: boolean;
}

interface Emits {
  (e: "update:show", value: boolean): void;
  (e: "confirm"): void;
  (e: "cancel"): void;
  (e: "close"): void;
}

const props = withDefaults(defineProps<Props>(), {
  title: "",
  confirmText: "",
  cancelText: "",
  loading: false,
  width: "800px",
  maxWidth: "95vw",
  height: "auto",
  maxHeight: "80vh",
  modalClass: "",
  cardClass: "",
  showCloseButton: true,
  closable: true,
  maskClosable: true,
  preventCloseOnLoading: true,
  footer: true,
});

const emit = defineEmits<Emits>();

const { t } = useI18n();

// 计算确认按钮文本
const confirmButtonText = computed(() => {
  return props.confirmText || t("common.confirm");
});

// 计算取消按钮文本
const cancelButtonText = computed(() => {
  return props.cancelText || t("common.cancel");
});

// 计算模态框样式
const modalStyle = computed(() => {
  const style: Record<string, string> = {};

  if (typeof props.width === "number") {
    style.width = `${props.width}px`;
  } else if (props.width) {
    style.width = props.width;
  }

  if (typeof props.height === "number") {
    style.height = `${props.height}px`;
  } else if (props.height && props.height !== "auto") {
    style.height = props.height;
  }

  if (props.maxWidth) {
    style.maxWidth = props.maxWidth;
  }

  if (props.maxHeight) {
    style.maxHeight = props.maxHeight;
  }

  return style;
});

// 计算模态框是否可关闭
const canClose = computed(() => {
  return !props.loading || !props.preventCloseOnLoading;
});

// 处理模态框关闭
function handleClose() {
  if (!canClose.value) {
    return;
  }
  emit("update:show", false);
  emit("close");
}

// 处理取消
function handleCancel() {
  if (!canClose.value) {
    return;
  }
  emit("cancel");
  handleClose();
}

// 处理确认
function handleConfirm() {
  if (props.loading) {
    return;
  }
  emit("confirm");
}
</script>

<template>
  <n-modal
    :show="show"
    @update:show="handleClose"
    :mask-closable="maskClosable && canClose"
    :class="['base-modal', modalClass]"
  >
    <div
      :class="['liquid-glass-modal-card base-modal-card', cardClass]"
      role="dialog"
      aria-modal="true"
      :style="modalStyle"
    >
      <!-- 头部 -->
      <div class="modal-card-header" v-if="title || showCloseButton">
        <span class="modal-card-title" v-if="title">{{ title }}</span>
        <n-button
          v-if="showCloseButton"
          quaternary
          circle
          @click="handleClose"
          :disabled="!canClose"
          class="modal-close-button"
        >
          <template #icon>
            <n-icon :component="Close" :size="16" />
          </template>
        </n-button>
      </div>

      <!-- 内容 -->
      <div class="modal-card-content">
        <slot />
      </div>

      <!-- 底部 -->
      <div class="modal-card-footer" v-if="footer">
        <n-button @click="handleCancel" :disabled="!canClose" class="btn-cancel">
          {{ cancelButtonText }}
        </n-button>
        <n-button
          type="primary"
          @click="handleConfirm"
          :loading="loading"
          :disabled="loading"
          class="btn-confirm"
        >
          {{ confirmButtonText }}
        </n-button>
      </div>
    </div>
  </n-modal>
</template>

<style scoped>
.base-modal {
  /* 基础模态框样式 */
}

.liquid-glass-modal-card {
  /* 液态玻璃风格 - 统一标准 */
  background: rgba(250, 252, 255, 0.9);
  backdrop-filter: blur(40px) saturate(180%);
  -webkit-backdrop-filter: blur(40px) saturate(180%);
  border: 1px solid rgba(255, 255, 255, 0.5);
  border-radius: var(--radius-md, 12px);
  box-shadow:
    0 0 0 1px rgba(255, 255, 255, 0.5),
    0 4px 24px rgba(0, 0, 0, 0.1),
    0 16px 64px rgba(0, 0, 0, 0.08),
    0 32px 128px rgba(0, 0, 0, 0.06);
  display: flex;
  flex-direction: column;
}

/* 暗黑模式 - 统一标准 */
html.dark .liquid-glass-modal-card {
  background: rgba(35, 40, 55, 0.95);
  border: 1px solid rgba(255, 255, 255, 0.18);
  box-shadow:
    0 0 0 1px rgba(255, 255, 255, 0.25),
    0 4px 24px rgba(0, 0, 0, 0.3),
    0 16px 64px rgba(0, 0, 0, 0.2),
    0 32px 128px rgba(0, 0, 0, 0.15);
}

.base-modal-card {
  border-radius: var(--border-radius-lg);
  box-shadow: var(--shadow-lg);
  transition: all 0.3s ease;
  animation: modalSlideIn 0.3s ease-out;
}

@keyframes modalSlideIn {
  from {
    opacity: 0;
    transform: translateY(-20px) scale(0.95);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

.modal-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid rgba(239, 239, 245, 0.8);
  flex-shrink: 0;
}

html.dark .modal-card-header {
  border-bottom: 1px solid rgba(255, 255, 255, 0.12);
}

.modal-card-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
}

.modal-card-content {
  padding: 20px;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  max-height: v-bind('props.maxHeight || "80vh"');
}

.modal-card-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding: 16px 20px;
  border-top: 1px solid rgba(239, 239, 245, 0.8);
  flex-shrink: 0;
}

html.dark .modal-card-footer {
  border-top: 1px solid rgba(255, 255, 255, 0.12);
}

/* 响应式适配 */
@media (max-width: 768px) {
  .base-modal-card {
    width: 95vw;
    max-width: 95vw;
    margin: 0;
    border-radius: var(--border-radius-md);
  }

  .modal-card-header {
    padding: 12px 16px;
  }

  .modal-card-content {
    padding: 16px;
    max-height: 70vh;
  }

  .modal-card-footer {
    padding: 12px 16px;
    flex-direction: column-reverse;
    gap: 8px;
  }

  .modal-card-footer .n-button {
    width: 100%;
  }
}

/* 超小屏幕适配 */
@media (max-width: 480px) {
  .base-modal-card {
    width: 98vw;
    max-width: 98vw;
  }

  .modal-card-header {
    padding: 10px 12px;
  }

  .modal-card-content {
    padding: 12px;
    max-height: 65vh;
  }

  .modal-card-footer {
    padding: 10px 12px;
  }
}

/* 暗黑模式适配 - 已在 .liquid-glass-modal-card 中处理 */

/* 禁用关闭按钮缩放动画以避免点击冲突 */
.modal-close-button:active {
  transform: none !important;
}
</style>
