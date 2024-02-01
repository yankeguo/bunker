<script setup lang="ts">
import type { FormError, FormSubmitEvent } from "#ui/types";
import { guardWorking } from "~/composables/error";

definePageMeta({
  middleware: ["auth"],
});

const { data: users, refresh: refreshUsers } = await useUsers();

const columns = [
  {
    key: "id",
    label: "Username",
  },
  {
    key: "role",
  },
  {
    key: 'actions-admin'
  },
  {
    key: 'actions-blocked'
  }
];

const state = reactive<{
  id?: string;
  password?: string;
}>({
  id: undefined,
  password: undefined,
});

const validate = (state: any): FormError[] => {
  const errors = [];
  if (!state.id) errors.push({ path: "id", message: "Required" });
  if (!state.password) errors.push({ path: "password", message: "Required" });
  return errors;
};

const working = ref(0);

async function onSubmit(event: FormSubmitEvent<any>) {
  await guardWorking(working, async () => {

    await $fetch("/backend/users/create", {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(event.data)
    })

    await refreshUsers()

  })
}

async function updateUser(id: string, { is_admin, is_blocked }: { is_admin?: boolean, is_blocked?: boolean }) {
  let message = 'Confirm to ';

  if (typeof is_admin === 'boolean') {
    message += is_admin ? 'set admin' : 'unset admin'
  }

  if (typeof is_blocked === 'boolean') {
    message += is_blocked ? 'block' : 'unblock'
  }

  message += ` for user ${id}?`

  if (!confirm(message)) {
    return
  }

  await guardWorking(working, async () => {

    await $fetch("/backend/users/update", {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ id, is_admin, is_blocked })
    })

    await refreshUsers()

  })
}
</script>

<template>
  <SkeletonDashboard title-name="Users" title-icon="i-mdi-account-multiple">
    <div class="w-1/3">
      <UCard :ui="{
        body: {
          padding: 'p-1',
        },
        header: {
          padding: 'p-1',
        },
      }">
        <template #header>
          <div class="flex flex-row items-center">
            <UIcon name="i-mdi-server-plus" class="me-1"></UIcon>
            <span>Add / Update User</span>
          </div>
        </template>
        <UForm :validate="validate" :state="state" class="space-y-4" @submit="onSubmit">
          <UFormGroup label="Name" name="id">
            <UInput v-model="state.id" />
          </UFormGroup>

          <UFormGroup label="Password" name="password">
            <UInput v-model="state.password" type="password" />
          </UFormGroup>

          <UButton type="submit" icon="i-mdi-check-circle" label="Submit" :loading="!!working" :disabled="!!working">
          </UButton>
        </UForm>
      </UCard>
    </div>
    <UTable class="mt-4" :rows="users.users" :columns="columns">
      <template #id-data="{ row }">
        <span class="font-semibold">{{ row.id }}</span>
      </template>
      <template #role-data="{ row }">
        <UBadge color="red" class="me-2" v-if="row.is_blocked">Disabled</UBadge>
        <UBadge variant="outline" color="lime" v-else-if="row.is_admin">Admin</UBadge>
        <UBadge variant="outline" v-else>Normal</UBadge>
      </template>
      <template #actions-admin-data="{ row }">
        <template v-if="!row.is_blocked">
          <UButton v-if="row.is_admin" variant="link" color="red" icon="i-mdi-account-tie-voice-off" label="Revoke Admin"
            @click="updateUser(row.id, { is_admin: false })" :disabled="!!working" :loading="!!working"></UButton>
          <UButton v-else variant="link" color="lime" icon="i-mdi-account-tie-voice" label="Set Admin"
            @click="updateUser(row.id, { is_admin: true })" :disabled="!!working" :loading="!!working"></UButton>
        </template>
      </template>

      <template #actions-blocked-data="{ row }">
        <UButton v-if="row.is_blocked" variant="link" color="lime" icon="i-mdi-account-check" label="Enable"
          @click="updateUser(row.id, { is_blocked: false })" :disabled="!!working" :loading="!!working"></UButton>
        <UButton v-else variant="link" color="red" icon="i-mdi-account-cancel" label="Disable"
          @click="updateUser(row.id, { is_blocked: true })" :disabled="!!working" :loading="!!working"></UButton>
      </template>
    </UTable>
  </SkeletonDashboard>
</template>
