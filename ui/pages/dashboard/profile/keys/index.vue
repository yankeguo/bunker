<script setup lang="ts">
definePageMeta({
  middleware: ["auth"],
});

const columns = [
  {
    key: "display_name",
    label: "Name",
  },
  {
    key: "id",
    label: "Fingerprint",
  },
  {
    key: "actions",
  },
];

const { data: keys, refresh: refreshKeys } = await useKeys();

const deletionWorking = ref(false);

async function deleteKey(id: string) {
  if (!confirm("Are you sure you want to delete this key?")) {
    return;
  }
  deletionWorking.value = true;
  try {
    await $fetch("/backend/keys/delete", {
      method: "POST",
      body: JSON.stringify({ id }),
    });
  } catch (e: any) {
    handleError(e);
  } finally {
    deletionWorking.value = false;
  }
  refreshKeys();
}
</script>

<template>
  <SkeletonDashboard title-name="SSH Keys" title-icon="i-mdi-key-chain">
    <div class="mb-4">
      <UButton
        :to="{ name: 'dashboard-profile-keys-new' }"
        icon="i-mdi-plus"
        label="Add SSH Key"
      ></UButton>
    </div>
    <UTable :rows="keys.keys" :columns="columns">
      <template #actions-data="{ row }">
        <UButton
          variant="link"
          color="red"
          icon="i-mdi-trash"
          label="Delete"
          @click="deleteKey(row.id)"
        ></UButton>
      </template>
    </UTable>
  </SkeletonDashboard>
</template>
