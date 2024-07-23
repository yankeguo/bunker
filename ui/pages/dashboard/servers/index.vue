<script setup lang="ts">
import type { FormError, FormSubmitEvent } from "#ui/types";
import { guardWorking } from "~/composables/error";

const { $t } = useNuxtApp();

definePageMeta({
  middleware: ["auth"],
});

const { data: servers, refresh: refreshServers } = await useServers();

const columns = [
  {
    key: "id",
    label: $t('common.server_id'),
  },
  {
    key: "address",
    label: $t('common.server_address'),
  },
  {
    key: 'actions'
  }
];

const state = reactive<{
  id?: string;
  address?: string;
}>({
  id: undefined,
  address: undefined,
});

const validate = (state: any): FormError[] => {
  const errors = [];
  if (!state.id) errors.push({ path: "id", message: "Required" });
  if (!state.address) errors.push({ path: "address", message: "Required" });
  return errors;
};

const working = ref(0);

async function onSubmit(event: FormSubmitEvent<any>) {
  await guardWorking(working, async () => {

    await $fetch("/backend/servers/create", {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(event.data)
    })

    await refreshServers()

  })
}

async function editServer({ id, address }: { id: string, address: string }) {
  state.id = id
  state.address = address
}

async function deleteServer(id: string) {
  if (!confirm(`confirm to to delete server ${id}?`)) {
    return
  }

  await guardWorking(working, async () => {

    await $fetch("/backend/servers/delete", {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ id })
    })

    await refreshServers()

  })
}
</script>

<template>
  <SkeletonDashboard :title-name="$t('servers.title')" title-icon="i-mdi-server">
    <template #left>
      <UCard :ui="uiCard">
        <template #header>
          <div class="flex flex-row items-center">
            <UIcon name="i-mdi-server-plus" class="me-1"></UIcon>
            <span>{{ $t('servers.add_update_server') }}</span>
          </div>
        </template>
        <UForm :validate="validate" :state="state" class="space-y-4" @submit="onSubmit">
          <UFormGroup :label="$t('common.server_id')" name="id">
            <UInput v-model="state.id" :placeholder="$t('servers.input_server_id')" />
          </UFormGroup>

          <UFormGroup :label="$t('common.server_address')" name="address">
            <UInput v-model="state.address" :placeholder="$t('servers.input_server_address')" />
          </UFormGroup>

          <UButton type="submit" icon="i-mdi-check-circle" :label="$t('common.submit')" :loading="!!working"
            :disabled="!!working">
          </UButton>
        </UForm>
      </UCard>

      <div class="pt-8">
        <UCard :ui="uiCard">
          <article class="prose dark:prose-invert" v-html="$t('servers.intro_authorized_keys')"></article>
          <template #footer>
            <UButton variant="link" to="/backend/authorized_keys" target="_blank"
              :label="$t('servers.view_authorized_keys')">
              <template #trailing>
                <UIcon name="i-heroicons-arrow-right-20-solid" />
              </template>
            </UButton>
          </template>
        </UCard>
      </div>
    </template>

    <UTable :rows="servers.servers" :columns="columns">
      <template #actions-data="{ row }">
        <UButton variant="link" color="blue" icon="i-mdi-edit" :label="$t('common.edit')" @click="editServer(row)"
          :disabled="!!working" :loading="!!working"></UButton>

        <UButton variant="link" color="red" icon="i-mdi-trash" :label="$t('common.delete')" @click="deleteServer(row.id)"
          :disabled="!!working" :loading="!!working"></UButton>
      </template>

    </UTable>
  </SkeletonDashboard>
</template>
