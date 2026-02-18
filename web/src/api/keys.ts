import i18n from "@/locales";
import type {
  APIKey,
  Group,
  GroupConfigOption,
  GroupStatsResponse,
  KeyStatus,
  ParentAggregateGroup,
  TaskInfo,
} from "@/types/models";
import http from "@/utils/http";

export const keysApi = {
  // Get all groups
  async getGroups(): Promise<Group[]> {
    const res = await http.get("/groups");
    return res.data || [];
  },

  // Create group
  async createGroup(group: Partial<Group>): Promise<Group> {
    const res = await http.post("/groups", group);
    return res.data;
  },

  // Update group
  async updateGroup(groupId: number, group: Partial<Group>): Promise<Group> {
    const res = await http.put(`/groups/${groupId}`, group);
    return res.data;
  },

  // Delete group
  deleteGroup(groupId: number): Promise<void> {
    return http.delete(`/groups/${groupId}`);
  },

  // Get group statistics
  async getGroupStats(groupId: number): Promise<GroupStatsResponse> {
    const res = await http.get(`/groups/${groupId}/stats`);
    return res.data;
  },

  // Get group configuration options
  async getGroupConfigOptions(): Promise<GroupConfigOption[]> {
    const res = await http.get("/groups/config-options");
    return res.data || [];
  },

  // Copy group
  async copyGroup(
    groupId: number,
    copyData: {
      copy_keys: "none" | "valid_only" | "all";
    }
  ): Promise<{
    group: Group;
  }> {
    const res = await http.post(`/groups/${groupId}/copy`, copyData, {
      hideMessage: true,
    });
    return res.data;
  },

  // Get group list
  async listGroups(): Promise<Pick<Group, "id" | "name" | "display_name">[]> {
    const res = await http.get("/groups/list");
    return res.data || [];
  },

  // Get group keys list
  async getGroupKeys(params: {
    group_id: number;
    page: number;
    page_size: number;
    key_value?: string;
    status?: KeyStatus;
  }): Promise<{
    items: APIKey[];
    pagination: {
      total_items: number;
      total_pages: number;
    };
  }> {
    const res = await http.get("/keys", { params });
    return res.data;
  },

  // Batch add keys - deprecated
  async addMultipleKeys(
    group_id: number,
    keys_text: string
  ): Promise<{
    added_count: number;
    ignored_count: number;
    total_in_group: number;
  }> {
    const res = await http.post("/keys/add-multiple", {
      group_id,
      keys_text,
    });
    return res.data;
  },

  // Async batch add keys
  async addKeysAsync(group_id: number, keys_text?: string, file?: File): Promise<TaskInfo> {
    let requestData: FormData | { group_id: number; keys_text: string };
    const config: { hideMessage: boolean; headers?: { "Content-Type": string } } = {
      hideMessage: true,
    };

    if (file) {
      // File upload mode
      const formData = new FormData();
      formData.append("group_id", group_id.toString());
      formData.append("file", file);
      requestData = formData;
      config.headers = { "Content-Type": "multipart/form-data" };
    } else {
      // Text input mode
      requestData = { group_id, keys_text: keys_text || "" };
    }

    const res = await http.post("/keys/add-async", requestData, config);
    return res.data;
  },

  // Update key notes
  async updateKeyNotes(keyId: number, notes: string): Promise<void> {
    await http.put(`/keys/${keyId}/notes`, { notes }, { hideMessage: true });
  },

  // Test keys
  async testKeys(
    group_id: number,
    keys_text: string,
    model?: string
  ): Promise<{
    results: {
      key_value: string;
      is_valid: boolean;
      error: string;
    }[];
    total_duration: number;
  }> {
    const res = await http.post(
      "/keys/test-multiple",
      {
        group_id,
        keys_text,
        model,
      },
      {
        hideMessage: true,
      }
    );
    return res.data;
  },

  // Test next key (using polling mechanism)
  async testNextKey(
    group_id: number,
    model?: string
  ): Promise<{
    result: {
      key_value: string;
      is_valid: boolean;
      error: string;
    };
    total_duration: number;
  }> {
    const res = await http.post(
      "/keys/test-next",
      {
        group_id,
        model,
      },
      {
        hideMessage: true,
      }
    );
    return res.data;
  },

  // Delete keys
  async deleteKeys(
    group_id: number,
    keys_text: string
  ): Promise<{ deleted_count: number; ignored_count: number; total_in_group: number }> {
    const res = await http.post("/keys/delete-multiple", {
      group_id,
      keys_text,
    });
    return res.data;
  },

  // Async batch delete keys
  async deleteKeysAsync(group_id: number, keys_text: string): Promise<TaskInfo> {
    const res = await http.post(
      "/keys/delete-async",
      {
        group_id,
        keys_text,
      },
      {
        hideMessage: true,
      }
    );
    return res.data;
  },

  // Restore keys
  restoreKeys(group_id: number, keys_text: string): Promise<null> {
    return http.post("/keys/restore-multiple", {
      group_id,
      keys_text,
    });
  },

  // Restore all invalid keys
  restoreAllInvalidKeys(group_id: number): Promise<void> {
    return http.post("/keys/restore-all-invalid", { group_id });
  },

  // Clear all invalid keys
  clearAllInvalidKeys(group_id: number): Promise<{ data: { message: string } }> {
    return http.post(
      "/keys/clear-all-invalid",
      { group_id },
      {
        hideMessage: true,
      }
    );
  },

  // Clear all keys
  clearAllKeys(group_id: number): Promise<{ data: { message: string } }> {
    return http.post(
      "/keys/clear-all",
      { group_id },
      {
        hideMessage: true,
      }
    );
  },

  // Export keys
  exportKeys(groupId: number, status: "all" | "active" | "invalid" = "all"): void {
    const authKey = localStorage.getItem("authKey");
    if (!authKey) {
      window.$message.error(i18n.global.t("auth.noAuthKeyFound"));
      return;
    }

    const params = new URLSearchParams({
      group_id: groupId.toString(),
      key: authKey,
    });

    if (status !== "all") {
      params.append("status", status);
    }

    const url = `${http.defaults.baseURL}/keys/export?${params.toString()}`;

    const link = document.createElement("a");
    link.href = url;
    link.setAttribute("download", `keys-group_${groupId}-${status}-${Date.now()}.txt`);
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  },

  // Validate group keys
  async validateGroupKeys(
    groupId: number,
    status?: "active" | "invalid"
  ): Promise<{
    is_running: boolean;
    group_name: string;
    processed: number;
    total: number;
    started_at: string;
  }> {
    const payload: { group_id: number; status?: string } = { group_id: groupId };
    if (status) {
      payload.status = status;
    }
    const res = await http.post("/keys/validate-group", payload);
    return res.data;
  },

  // Get task status
  async getTaskStatus(): Promise<TaskInfo> {
    const res = await http.get("/tasks/status");
    return res.data;
  },

  // Get subgroups of aggregate group
  async getSubGroups(aggregateGroupId: number): Promise<import("@/types/models").SubGroupInfo[]> {
    const res = await http.get(`/groups/${aggregateGroupId}/sub-groups`);
    return res.data || [];
  },

  // Add subgroups to aggregate group
  async addSubGroups(
    aggregateGroupId: number,
    subGroups: { group_id: number; weight: number }[]
  ): Promise<void> {
    await http.post(`/groups/${aggregateGroupId}/sub-groups`, {
      sub_groups: subGroups,
    });
  },

  // Update subgroup weight
  async updateSubGroupWeight(
    aggregateGroupId: number,
    subGroupId: number,
    weight: number
  ): Promise<void> {
    await http.put(`/groups/${aggregateGroupId}/sub-groups/${subGroupId}/weight`, {
      weight,
    });
  },

  // Delete subgroup
  async deleteSubGroup(aggregateGroupId: number, subGroupId: number): Promise<void> {
    await http.delete(`/groups/${aggregateGroupId}/sub-groups/${subGroupId}`);
  },

  // Get parent aggregate groups list
  async getParentAggregateGroups(groupId: number): Promise<ParentAggregateGroup[]> {
    const res = await http.get(`/groups/${groupId}/parent-aggregate-groups`);
    return res.data || [];
  },
};
