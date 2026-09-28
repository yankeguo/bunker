<script setup lang="ts">
import type { FormError, FormSubmitEvent } from "#ui/types";
import { guardWorking } from "~/composables/error";
import { fill } from "~/utils/format";

const { $t } = useNuxtApp();

definePageMeta({
  middleware: ["auth", "admin"],
});

const { data: servers, refresh: refreshServers } = await useServers();
const { data: hostKeys, refresh: refreshHostKeys } = await useHostKeys();

const hostKeysByServer = computed(() => {
  const grouped: Record<string, BHostKey[]> = {};
  for (const key of hostKeys.value.host_keys || []) {
    (grouped[key.server_id] ||= []).push(key);
  }
  return grouped;
});

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
    key: "host_key",
    label: $t('servers.host_key'),
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
  if (!state.id) errors.push({ path: "id", message: $t("common.required") });
  if (!state.address) errors.push({ path: "address", message: $t("common.required") });
  return errors;
};

const working = ref(0);

async function onSubmit(event: FormSubmitEvent<any>) {
  await guardWorking(working, async () => {

    await $fetch("/backend/servers/create", {
      method: 'POST',
      body: event.data,
    })

    await refreshServers()

  })
}

async function editServer({ id, address }: { id: string, address: string }) {
  state.id = id
  state.address = address
}

async function deleteServer(id: string) {
  if (!confirm(fill($t('servers.confirm_delete'), { id }))) {
    return
  }

  await guardWorking(working, async () => {

    await $fetch("/backend/servers/delete", {
      method: 'POST',
      body: { id },
    })

    await refreshServers()
    await refreshHostKeys()

  })
}

async function resetHostKeys(id: string) {
  if (!confirm(fill($t('servers.confirm_reset_host_key'), { id }))) {
    return
  }

  await guardWorking(working, async () => {
    await $fetch("/backend/host_keys/delete", {
      method: 'POST',
      body: { server_id: id },
    })
    await refreshHostKeys()
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
      <template #host_key-data="{ row }">
        <div v-if="hostKeysByServer[row.id]?.length" class="space-y-1">
          <div v-for="key in hostKeysByServer[row.id]" :key="key.id" class="font-mono text-xs break-all">
            {{ key.key_type }} {{ key.fingerprint }}
          </div>
          <UButton size="2xs" variant="ghost" color="red" :label="$t('servers.reset_host_key')"
            :disabled="!!working" @click="resetHostKeys(row.id)" />
        </div>
        <span v-else class="text-sm text-gray-500">{{ $t('servers.host_key_empty') }}</span>
      </template>

      <template #actions-data="{ row }">
        <UButton variant="link" color="blue" icon="i-mdi-edit" :label="$t('common.edit')" @click="editServer(row)"
          :disabled="!!working" :loading="!!working"></UButton>

        <UButton variant="link" color="red" icon="i-mdi-trash" :label="$t('common.delete')" @click="deleteServer(row.id)"
          :disabled="!!working" :loading="!!working"></UButton>
      </template>
      <template #empty-state>
        <div class="py-6 text-center text-sm text-gray-500">{{ $t('common.empty') }}</div>
      </template>

    </UTable>
  </SkeletonDashboard>
</template>
