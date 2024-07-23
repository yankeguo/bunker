<script setup lang="ts">
import type { FormError, FormSubmitEvent } from "#ui/types";
import { guardWorking } from "~/composables/error";

const { $t } = useNuxtApp()

definePageMeta({
  middleware: ["auth"],
});

const { data: users, refresh: refreshUsers } = await useUsers();

const columns = [
  {
    key: "id",
    label: $t('common.user_id'),
  },
  {
    key: "role",
    label: $t('common.user_role'),
  },
  {
    key: 'actions'
  },
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
  <SkeletonDashboard :title-name="$t('users.title')" title-icon="i-mdi-account-multiple">
    <template #left>
      <UCard :ui="uiCard">
        <template #header>
          <div class="flex flex-row items-center">
            <UIcon name="i-mdi-user-plus" class="me-1"></UIcon>
            <span>{{ $t('users.add_update_user') }}</span>
          </div>
        </template>
        <UForm :validate="validate" :state="state" class="space-y-4" @submit="onSubmit">
          <UFormGroup :label="$t('common.user_id')" name="id">
            <UInput v-model="state.id" :placeholder="$t('users.input_user_id')" />
          </UFormGroup>

          <UFormGroup :label="$t('common.password')" name="password">
            <UInput v-model="state.password" type="password" :placeholder="$t('users.input_password')" />
          </UFormGroup>

          <UButton type="submit" icon="i-mdi-check-circle" :label="$t('common.submit')" :loading="!!working"
            :disabled="!!working">
          </UButton>
        </UForm>
      </UCard>
    </template>

    <UTable :rows="users.users" :columns="columns">
      <template #id-data="{ row }">
        <UButton class="font-semibold" variant="link"
          :to="{ name: 'dashboard-users-user_id', params: { user_id: row.id } }" :label="row.id">
        </UButton>
      </template>
      <template #role-data="{ row }">
        <UBadge color="red" class="me-2" v-if="row.is_blocked">{{ $t('common.user_role_disabled') }}</UBadge>
        <UBadge variant="outline" color="lime" v-else-if="row.is_admin">{{ $t('common.user_role_admin') }}</UBadge>
        <UBadge variant="outline" v-else>{{ $t('common.user_role_standard') }}</UBadge>
      </template>
      <template #actions-data="{ row }">
        <template v-if="!row.is_blocked">
          <UButton class="w-30" v-if="row.is_admin" variant="ghost" color="red" icon="i-mdi-account-tie-voice-off"
            :label="$t('users.revoke_admin')" @click="updateUser(row.id, { is_admin: false })" :disabled="!!working"
            :loading="!!working"></UButton>
          <UButton class="w-30" v-else variant="ghost" color="lime" icon="i-mdi-account-tie-voice"
            :label="$t('users.assign_admin')" @click="updateUser(row.id, { is_admin: true })" :disabled="!!working"
            :loading="!!working"></UButton>
        </template>

        <UButton class="ms-2 w-20" v-if="row.is_blocked" variant="ghost" color="lime" icon="i-mdi-account-check"
          :label="$t('users.enable')" @click="updateUser(row.id, { is_blocked: false })" :disabled="!!working"
          :loading="!!working">
        </UButton>
        <UButton class="ms-2 w-20" v-else variant="ghost" color="red" icon="i-mdi-account-cancel"
          :label="$t('users.disable')" @click="updateUser(row.id, { is_blocked: true })" :disabled="!!working"
          :loading="!!working"></UButton>
      </template>
    </UTable>
  </SkeletonDashboard>
</template>
