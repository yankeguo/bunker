<script setup lang="ts">
import type { FormError, FormSubmitEvent } from "#ui/types";
import { guardWorking } from "~/composables/error";
import { fill } from "~/utils/format";

const { $t } = useNuxtApp()

definePageMeta({
  middleware: ["auth", "admin"],
});

const { data: users, refresh: refreshUsers } = await useUsers();
const { data: currentUser } = await useCurrentUser();

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
  if (!state.id) errors.push({ path: "id", message: $t("common.required") });
  if (!state.password) errors.push({ path: "password", message: $t("common.required") });
  return errors;
};

const working = ref(0);

async function onSubmit(event: FormSubmitEvent<any>) {
  await guardWorking(working, async () => {

    await $fetch("/backend/users/create", {
      method: 'POST',
      body: event.data,
    })

    state.password = undefined
    await refreshUsers()

  })
}

async function updateUser(id: string, { is_admin, is_blocked }: { is_admin?: boolean, is_blocked?: boolean }) {
  const actions = [];

  if (typeof is_admin === 'boolean') {
    actions.push(is_admin ? $t('users.assign_admin') : $t('users.revoke_admin'))
  }

  if (typeof is_blocked === 'boolean') {
    actions.push(is_blocked ? $t('users.disable') : $t('users.enable'))
  }

  if (!confirm(fill($t('users.confirm_update'), { actions: actions.join(', '), id }))) {
    return
  }

  await guardWorking(working, async () => {

    await $fetch("/backend/users/update", {
      method: 'POST',
      body: { id, is_admin, is_blocked },
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
          :to="{ name: 'dashboard-users-detail', query: { user_id: row.id } }" :label="row.id">
        </UButton>
      </template>
      <template #role-data="{ row }">
        <UBadge color="red" class="me-2" v-if="row.is_blocked">{{ $t('common.user_role_disabled') }}</UBadge>
        <UBadge variant="outline" color="lime" v-else-if="row.is_admin">{{ $t('common.user_role_admin') }}</UBadge>
        <UBadge variant="outline" v-else>{{ $t('common.user_role_standard') }}</UBadge>
      </template>
      <template #actions-data="{ row }">
        <span v-if="row.id === currentUser.user?.id" class="text-sm text-gray-500">{{ $t('users.current') }}</span>
        <template v-else>
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
      </template>
      <template #empty-state>
        <div class="py-6 text-center text-sm text-gray-500">{{ $t('common.empty') }}</div>
      </template>
    </UTable>
  </SkeletonDashboard>
</template>
