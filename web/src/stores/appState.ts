import { defineStore } from "pinia";

interface AppState {
  loading: boolean;
  taskPollingTrigger: number;
  lastCompletedTask: {
    groupName: string;
    taskType: string;
    finishedAt: string;
  } | null;
  groupDataRefreshTrigger: number;
  syncOperationTrigger: number;
  lastSyncOperation: {
    groupName: string;
    operationType: string;
    finishedAt: string;
  } | null;
}

export const useAppStateStore = defineStore("appState", {
  state: (): AppState => ({
    loading: false,
    taskPollingTrigger: 0,
    lastCompletedTask: null,
    groupDataRefreshTrigger: 0,
    syncOperationTrigger: 0,
    lastSyncOperation: null,
  }),

  actions: {
    triggerTaskPolling() {
      this.taskPollingTrigger++;
    },

    setLastCompletedTask(task: AppState["lastCompletedTask"]) {
      this.lastCompletedTask = task;
    },

    triggerGroupDataRefresh() {
      this.groupDataRefreshTrigger++;
    },

    triggerSyncOperationRefresh(groupName: string, operationType: string) {
      this.lastSyncOperation = {
        groupName,
        operationType,
        finishedAt: new Date().toISOString(),
      };
      this.syncOperationTrigger++;
    },

    setLoading(loading: boolean) {
      this.loading = loading;
    },
  },
});
