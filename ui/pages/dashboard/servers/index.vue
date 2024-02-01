<script setup lang="ts">
import type { FormError, FormSubmitEvent } from "#ui/types";
import { guardWorking } from "~/composables/error";

definePageMeta({
  middleware: ["auth"],
});

const { data: servers, refresh: refreshServers } = await useServers();

const columns = [
  {
    key: "id",
    label: "Name",
  },
  {
    key: "address",
    label: "Address",
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
  <SkeletonDashboard title-name="Servers" title-icon="i-mdi-server">
    <template #left>
      <UCard :ui="uiCard">
        <template #header>
          <div class="flex flex-row items-center">
            <UIcon name="i-mdi-server-plus" class="me-1"></UIcon>
            <span>Add / Update Server</span>
          </div>
        </template>
        <UForm :validate="validate" :state="state" class="space-y-4" @submit="onSubmit">
          <UFormGroup label="Name" name="id">
            <UInput v-model="state.id" placeholder="Input server name" />
          </UFormGroup>

          <UFormGroup label="Address" name="address">
            <UInput v-model="state.address" placeholder="Input server address" />
          </UFormGroup>

          <UButton type="submit" icon="i-mdi-check-circle" label="Submit" :loading="!!working" :disabled="!!working">
          </UButton>
        </UForm>
      </UCard>

      <div class="pt-8">
        <UCard :ui="uiCard">
          <p>
            Add the following public key to the server's authorized_keys file.
          </p>
          <template #footer>
            <UButton variant="link" to="/backend/authorized_keys" target="_blank" label="Server Authorized Keys">
              <template #trailing>
                <UIcon name="i-heroicons-arrow-right-20-solid" />
              </template>
            </UButton>
          </template>
        </UCard>
      </div>
    </template>

    <UTable class="mt-4" :rows="servers.servers" :columns="columns">
      <template #actions-data="{ row }">
        <UButton variant="link" color="blue" icon="i-mdi-edit" label="Edit" @click="editServer(row)" :disabled="!!working"
          :loading="!!working"></UButton>

        <UButton variant="link" color="red" icon="i-mdi-trash" label="Delete" @click="deleteServer(row.id)"
          :disabled="!!working" :loading="!!working"></UButton>
      </template>

    </UTable>
  </SkeletonDashboard>
</template>
