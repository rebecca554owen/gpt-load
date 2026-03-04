import { ref, type Ref } from "vue";
import type { MessageReactive, DialogReactive } from "naive-ui";
import { useI18n } from "vue-i18n";
import { keysApi } from "@/api/keys";
import { maskKey } from "@/utils/display";
import { useConfirmDialog } from "./useConfirmDialog";

export interface KeyRow {
  id: number;
  key_value: string;
  notes?: string;
  is_visible?: boolean;
}

export interface KeyActionsOptions {
  selectedGroup: { id: number; name: string } | null;
  onLoad: () => Promise<void>;
  dialog: (options: Record<string, unknown>) => DialogReactive;
}

export function useKeyActions(options: KeyActionsOptions) {
  const { t } = useI18n();
  const { isLoading: isDeleting, confirmAction: confirmDelete } = useConfirmDialog();
  const { isLoading: isRestoring, confirmAction: confirmRestore } = useConfirmDialog();

  const testingMsg: Ref<MessageReactive | null> = ref(null);

  async function testKey(key: KeyRow) {
    if (!options.selectedGroup?.id || !key.key_value || testingMsg.value) {
      return;
    }

    testingMsg.value = window.$message.info(t("keys.testingKey"), {
      duration: 0,
    });

    try {
      const response = await keysApi.testKeys(options.selectedGroup.id, key.key_value);
      const result = response.results?.[0] || {};

      testingMsg.value.destroy();
      testingMsg.value = null;

      if (result.is_valid) {
        window.$message.success(t("keys.testSuccess"));
      } else {
        window.$message.error(t("keys.testFailed"));
      }
      await options.onLoad();
    } catch (_error) {
      testingMsg.value?.destroy();
      testingMsg.value = null;
    }
  }

  async function deleteKey(key: KeyRow) {
    if (!options.selectedGroup?.id || !key.key_value) {
      return;
    }

    const groupId = options.selectedGroup.id;
    await confirmDelete(options.dialog, {
      title: t("keys.deleteKey"),
      content: t("keys.confirmDeleteKey", { key: maskKey(key.key_value) }),
      onConfirm: async () => {
        await keysApi.deleteKeys(groupId, key.key_value);
        await options.onLoad();
      },
    });
  }

  async function restoreKey(key: KeyRow) {
    if (!options.selectedGroup?.id || !key.key_value) {
      return;
    }

    const groupId = options.selectedGroup.id;
    await confirmRestore(options.dialog, {
      title: t("keys.restoreKey"),
      content: t("keys.confirmRestoreKey", { key: maskKey(key.key_value) }),
      onConfirm: async () => {
        await keysApi.restoreKeys(groupId, key.key_value);
        await options.onLoad();
      },
    });
  }

  async function restoreAllInvalid() {
    if (!options.selectedGroup?.id) {
      return;
    }

    const groupId = options.selectedGroup.id;
    await confirmRestore(options.dialog, {
      title: t("keys.restoreKeys"),
      content: t("keys.confirmRestoreAllInvalid"),
      onConfirm: async () => {
        await keysApi.restoreAllInvalidKeys(groupId);
        await options.onLoad();
      },
    });
  }

  async function clearAllInvalid() {
    if (!options.selectedGroup?.id) {
      return;
    }

    const groupId = options.selectedGroup.id;
    await confirmDelete(options.dialog, {
      title: t("keys.clearKeys"),
      content: t("keys.confirmClearInvalidKeys"),
      onConfirm: async () => {
        await keysApi.clearAllInvalidKeys(groupId);
        window.$message.success(t("keys.clearSuccess"));
        await options.onLoad();
      },
    });
  }

  async function validateKeys(status: "all" | "active" | "invalid") {
    if (!options.selectedGroup?.id || testingMsg.value) {
      return;
    }

    let statusText = t("common.all");
    if (status === "active") {
      statusText = t("keys.valid");
    } else if (status === "invalid") {
      statusText = t("keys.invalid");
    }

    testingMsg.value = window.$message.info(t("keys.validatingKeysMsg", { type: statusText }), {
      duration: 0,
    });

    try {
      await keysApi.validateGroupKeys(
        options.selectedGroup.id,
        status === "all" ? undefined : status
      );
      localStorage.removeItem("last_closed_task");
    } finally {
      testingMsg.value?.destroy();
      testingMsg.value = null;
    }
  }

  return {
    isDeleting,
    isRestoring,
    testingMsg,
    testKey,
    deleteKey,
    restoreKey,
    restoreAllInvalid,
    clearAllInvalid,
    validateKeys,
  };
}
