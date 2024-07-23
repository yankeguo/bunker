<script setup lang="ts">
import type { FormError, FormSubmitEvent } from "#ui/types";
import { guardWorking } from "~/composables/error";

const { $t } = useNuxtApp();

definePageMeta({
  middleware: ["auth"],
});

const columns = [
  {
    key: "display_name",
    label: $t('common.ssh_key_display_name'),
  },
  {
    key: "id",
    label: $t('common.ssh_key_id'),
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
  <SkeletonDashboard :title-name="$t('ssh_keys.title')" title-icon="i-mdi-key-chain">
    <template #left>
      <UCard :ui="uiCard">
        <template #header>
          <UIcon name="i-mdi-key-plus" class="me-1"></UIcon>
          <span>{{ $t('ssh_keys.add_ssh_key') }}</span>
        </template>
        <UForm :validate="validate" :state="state" class="space-y-4" @submit="onSubmit">
          <UFormGroup :label="$t('common.ssh_key_display_name')" name="display_name">
            <UInput v-model="state.display_name" :placeholder="$t('ssh_keys.input_display_name')" />
          </UFormGroup>

          <UFormGroup :label="$t('ssh_keys.public_key')" name="public_key">
            <UTextarea v-model="state.public_key" :rows="12" :placeholder="$t('ssh_keys.input_public_key')" />
          </UFormGroup>

          <UButton type="submit" :disabled="!!working" :loading="!!working" icon="i-mdi-check"
            :label="$t('common.submit')">
          </UButton>
        </UForm>
      </UCard>
    </template>

    <UTable :rows="keys.keys" :columns="columns">
      <template #actions-data="{ row }">
        <UButton variant="link" color="red" icon="i-mdi-trash" :label="$t('common.delete')" @click="deleteKey(row.id)">
        </UButton>
      </template>
    </UTable>

  </SkeletonDashboard>
</template>
