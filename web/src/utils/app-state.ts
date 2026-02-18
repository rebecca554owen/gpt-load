import { reactive } from "vue";

interface AppState {
  loading: boolean;
  taskPollingTrigger: number;
  groupDataRefreshTrigger: number;
  syncOperationTrigger: number;
  lastCompletedTask?: {
    groupName: string;
    taskType: string;
    finishedAt: string;
  };
  lastSyncOperation?: {
    groupName: string;
    operationType: string;
    finishedAt: string;
  };
}

export const appState = reactive<AppState>({
  loading: false,
  taskPollingTrigger: 0,
  groupDataRefreshTrigger: 0,
  syncOperationTrigger: 0,
});

// Trigger data refresh after sync operation
export function triggerSyncOperationRefresh(groupName: string, operationType: string) {
  appState.lastSyncOperation = {
    groupName,
    operationType,
    finishedAt: new Date().toISOString(),
  };
  appState.syncOperationTrigger++;
}
