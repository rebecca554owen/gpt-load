<script setup lang="ts">
import { keysApi } from "@/api/keys";
import EncryptionMismatchAlert from "@/components/EncryptionMismatchAlert.vue";
import GroupInfoCard from "@/components/keys/GroupInfoCard.vue";
import GroupList from "@/components/keys/GroupList.vue";
import KeyTable from "@/components/keys/KeyTable.vue";
import SubGroupTable from "@/components/keys/SubGroupTable.vue";
import ModelMappingTable from "@/components/keys/ModelMappingTable.vue";
import type { Group, ModelMapping, SubGroupInfo } from "@/types/models";
import { onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";

const groups = ref<Group[]>([]);
const loading = ref(false);
const selectedGroup = ref<Group | null>(null);
const subGroups = ref<SubGroupInfo[]>([]);
const loadingSubGroups = ref(false);
const router = useRouter();
const route = useRoute();

onMounted(async () => {
  await loadGroups();
});

async function loadGroups() {
  try {
    loading.value = true;
    groups.value = await keysApi.getGroups();
    // Select default group
    if (groups.value.length > 0 && !selectedGroup.value) {
      const groupId = route.query.groupId;
      const found = groups.value.find(g => String(g.id) === String(groupId));
      if (found) {
        selectedGroup.value = found;
      } else {
        handleGroupSelect(groups.value[0]);
      }
    }
  } catch (error) {
    console.error("Failed to load groups:", error);
    window.$message?.error("Failed to load group list");
  } finally {
    loading.value = false;
  }
}

async function loadSubGroups() {
  if (!selectedGroup.value?.id || selectedGroup.value.group_type !== "aggregate") {
    subGroups.value = [];
    return;
  }

  try {
    loadingSubGroups.value = true;
    subGroups.value = await keysApi.getSubGroups(selectedGroup.value.id);
  } catch (error) {
    console.error("Failed to load sub groups:", error);
    window.$message?.error("Failed to load subgroups");
    subGroups.value = [];
  } finally {
    loadingSubGroups.value = false;
  }
}

// Watch selected group changes, load subgroup data
watch(selectedGroup, async newGroup => {
  if (newGroup?.group_type === "aggregate") {
    await loadSubGroups();
  } else {
    subGroups.value = [];
  }
});

function handleGroupSelect(group: Group | null) {
  selectedGroup.value = group || null;
  if (String(group?.id) !== String(route.query.groupId)) {
    router.push({ name: "keys", query: { groupId: group?.id || "" } });
  }
}

async function refreshGroupsAndSelect(targetGroupId?: number, selectFirst = true) {
  await loadGroups();

  if (targetGroupId) {
    const targetGroup = groups.value.find(g => g.id === targetGroupId);
    if (targetGroup) {
      handleGroupSelect(targetGroup);
      return;
    }
  }

  if (selectedGroup.value) {
    const currentGroup = groups.value.find(g => g.id === selectedGroup.value?.id);
    if (currentGroup) {
      handleGroupSelect(currentGroup);
      if (currentGroup.group_type === "aggregate") {
        await loadSubGroups();
      }
      return;
    }
  }

  if (selectFirst && groups.value.length > 0) {
    handleGroupSelect(groups.value[0]);
  }
}

// Handle subgroup selection, navigate to corresponding group
function handleSubGroupSelect(groupId: number) {
  const targetGroup = groups.value.find(g => g.id === groupId);
  if (targetGroup) {
    handleGroupSelect(targetGroup);
  }
}

// Handle aggregate group navigation, navigate to corresponding aggregate group
function handleNavigateToGroup(groupId: number) {
  const targetGroup = groups.value.find(g => g.id === groupId);
  if (targetGroup) {
    handleGroupSelect(targetGroup);
  }
}

// Use API response data directly to update selectedGroup
function updateSelectedGroupFromResponse(updatedGroup: Group) {
  if (selectedGroup.value?.id === updatedGroup.id) {
    // Replace the entire object to ensure reactive update
    // Note: Backend returns model_mappings (array), need to map it to model_mappings_list
    const groupWithMappingsList = {
      ...selectedGroup.value,
      ...updatedGroup,
      // If backend returns model_mappings array, set it as model_mappings_list
      model_mappings_list:
        (updatedGroup.model_mappings as ModelMapping[] | undefined) ||
        updatedGroup.model_mappings_list,
    };
    selectedGroup.value = groupWithMappingsList;
  }
}
</script>

<template>
  <div>
    <!-- Encryption config error alert -->
    <encryption-mismatch-alert style="margin-bottom: 16px" />

    <div class="keys-container">
      <div class="sidebar">
        <group-list
          :groups="groups"
          :selected-group="selectedGroup"
          :loading="loading"
          @group-select="handleGroupSelect"
          @refresh="() => refreshGroupsAndSelect()"
          @refresh-and-select="id => refreshGroupsAndSelect(id)"
        />
      </div>

      <!-- Main content area on the right, 80% width -->
      <div class="main-content">
        <!-- Group info card, more compact -->
        <div class="group-info">
          <group-info-card
            :group="selectedGroup"
            :groups="groups"
            :sub-groups="subGroups"
            @refresh="() => refreshGroupsAndSelect()"
            @delete="() => refreshGroupsAndSelect(undefined, true)"
            @copy-success="group => refreshGroupsAndSelect(group.id)"
            @navigate-to-group="handleNavigateToGroup"
          />
        </div>

        <!-- Key table area / Subgroup list area / Model mapping area -->
        <div class="key-table-section">
          <!-- Standard group shows key list -->
          <key-table
            v-if="!selectedGroup || selectedGroup.group_type !== 'aggregate'"
            :selected-group="selectedGroup"
          />

          <!-- Aggregate group shows both subgroup list and model mapping list -->
          <template v-else>
            <!-- Subgroup table -->
            <sub-group-table
              :selected-group="selectedGroup"
              :sub-groups="subGroups"
              :groups="groups"
              :loading="loadingSubGroups"
              @refresh="loadSubGroups"
              @group-select="handleSubGroupSelect"
            />

            <!-- Model mapping list -->
            <div class="model-mapping-section">
              <model-mapping-table
                :selected-group="selectedGroup"
                :model-mappings="selectedGroup.model_mappings_list"
                :sub-groups="subGroups"
                :groups="groups"
                :loading="loadingSubGroups"
                @refresh="() => refreshGroupsAndSelect(selectedGroup?.id)"
                @group-select="handleSubGroupSelect"
                @group-updated="updateSelectedGroupFromResponse"
              />
            </div>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.keys-container {
  display: flex;
  flex-direction: column;
  gap: 12px;
  width: 100%;
}

.sidebar {
  width: 100%;
  flex-shrink: 0;
}

.main-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.group-info {
  flex-shrink: 0;
}

.key-table-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.model-mapping-section {
  margin-top: 16px;
  flex-shrink: 0;
}

@media (min-width: 768px) {
  .keys-container {
    flex-direction: row;
    gap: 16px;
  }

  .sidebar {
    width: 240px;
    height: calc(100vh - 139px);
  }
}
</style>
