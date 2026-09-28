<script setup lang="ts">
import type { FormError, FormSubmitEvent, TableColumn } from "@nuxt/ui";
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

const columns: TableColumn<any>[] = [
  {
    accessorKey: "id",
    header: $t("common.server_id"),
  },
  {
    accessorKey: "address",
    header: $t("common.server_address"),
  },
  {
    id: "host_key",
    header: $t("servers.host_key"),
  },
  {
    id: "actions",
  },
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
  if (!state.id) errors.push({ name: "id", message: $t("common.required") });
  if (!state.address) errors.push({ name: "address", message: $t("common.required") });
  return errors;
};

const working = ref(0);

async function onSubmit(event: FormSubmitEvent<any>) {
  await guardWorking(working, async () => {
    await $fetch("/backend/servers/create", {
      method: "POST",
      body: event.data,
    });

    await refreshServers();
  });
}

async function editServer({ id, address }: { id: string; address: string }) {
  state.id = id;
  state.address = address;
}

async function deleteServer(id: string) {
  if (!confirm(fill($t("servers.confirm_delete"), { id }))) {
    return;
  }

  await guardWorking(working, async () => {
    await $fetch("/backend/servers/delete", {
      method: "POST",
      body: { id },
    });

    await refreshServers();
    await refreshHostKeys();
  });
}

async function resetHostKeys(id: string) {
  if (!confirm(fill($t("servers.confirm_reset_host_key"), { id }))) {
    return;
  }

  await guardWorking(working, async () => {
    await $fetch("/backend/host_keys/delete", {
      method: "POST",
      body: { server_id: id },
    });
    await refreshHostKeys();
  });
}
</script>

<template>
  <SkeletonDashboard :title-name="$t('servers.title')" title-icon="i-mdi-server">
    <template #left>
      <UCard :ui="uiCard">
        <template #header>
          <div class="flex flex-row items-center">
            <UIcon name="i-mdi-server-plus" class="me-1" />
            <span>{{ $t("servers.add_update_server") }}</span>
          </div>
        </template>
        <UForm :validate="validate" :state="state" class="space-y-4" @submit="onSubmit">
          <UFormField :label="$t('common.server_id')" name="id">
            <UInput v-model="state.id" :placeholder="$t('servers.input_server_id')" />
          </UFormField>

          <UFormField :label="$t('common.server_address')" name="address">
            <UInput v-model="state.address" :placeholder="$t('servers.input_server_address')" />
          </UFormField>

          <UButton
            type="submit"
            icon="i-mdi-check-circle"
            :label="$t('common.submit')"
            :loading="!!working"
            :disabled="!!working"
          />
        </UForm>
      </UCard>

      <div class="pt-8">
        <UCard :ui="uiCard">
          <article class="prose dark:prose-invert" v-html="$t('servers.intro_authorized_keys')"></article>
          <template #footer>
            <UButton
              variant="link"
              to="/backend/authorized_keys"
              target="_blank"
              :label="$t('servers.view_authorized_keys')"
              trailing-icon="i-heroicons-arrow-right-20-solid"
            />
          </template>
        </UCard>
      </div>
    </template>

    <UTable :data="servers.servers" :columns="columns">
      <template #host_key-cell="{ row }">
        <div v-if="hostKeysByServer[row.original.id]?.length" class="space-y-1">
          <div v-for="key in hostKeysByServer[row.original.id]" :key="key.id" class="font-mono text-xs break-all">
            {{ key.key_type }} {{ key.fingerprint }}
          </div>
          <UButton
            size="xs"
            variant="ghost"
            color="error"
            :label="$t('servers.reset_host_key')"
            :disabled="!!working"
            @click="resetHostKeys(row.original.id)"
          />
        </div>
        <span v-else class="text-sm text-muted">{{ $t("servers.host_key_empty") }}</span>
      </template>

      <template #actions-cell="{ row }">
        <UButton
          variant="link"
          color="info"
          icon="i-mdi-edit"
          :label="$t('common.edit')"
          :disabled="!!working"
          :loading="!!working"
          @click="editServer(row.original)"
        />

        <UButton
          variant="link"
          color="error"
          icon="i-mdi-trash"
          :label="$t('common.delete')"
          :disabled="!!working"
          :loading="!!working"
          @click="deleteServer(row.original.id)"
        />
      </template>
      <template #empty>
        <div class="py-6 text-center text-sm text-muted">{{ $t("common.empty") }}</div>
      </template>
    </UTable>
  </SkeletonDashboard>
</template>
