<script setup lang="ts">
import type { FormError, FormSubmitEvent } from "#ui/types";
import { guardWorking } from "~/composables/error";

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

const state = reactive({
  display_name: undefined,
  public_key: undefined,
});

const validate = (state: any): FormError[] => {
  const errors = [];
  if (!state.display_name)
    errors.push({ path: "display_name", message: "Required" });
  if (!state.public_key)
    errors.push({ path: "public_key", message: "Required" });
  return errors;
};

const working = ref(0);

async function onSubmit(event: FormSubmitEvent<any>) {
  return guardWorking(working, async () => {
    await $fetch("/backend/keys/create", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(event.data),
    });
    await refreshKeys()
  })
}
</script>

<template>
  <SkeletonDashboard title-name="SSH Keys" title-icon="i-mdi-key-chain">
    <template #left>
      <UCard :ui="uiCard">
        <template #header>
          <UIcon name="i-mdi-key-plus" class="me-1"></UIcon>
          <span>Add Key</span>
        </template>
        <UForm :validate="validate" :state="state" class="space-y-4" @submit="onSubmit">
          <UFormGroup label="Name" name="display_name">
            <UInput v-model="state.display_name" placeholder="Input name" />
          </UFormGroup>

          <UFormGroup label="Public Key" name="public_key">
            <UTextarea v-model="state.public_key" :rows="12" placeholder="Input ssh public key" />
          </UFormGroup>

          <UButton type="submit" :disabled="!!working" :loading="!!working" icon="i-mdi-check" label="Submit">
          </UButton>
        </UForm>
      </UCard>
    </template>

    <UTable :rows="keys.keys" :columns="columns">
      <template #actions-data="{ row }">
        <UButton variant="link" color="red" icon="i-mdi-trash" label="Delete" @click="deleteKey(row.id)"></UButton>
      </template>
    </UTable>

  </SkeletonDashboard>
</template>
