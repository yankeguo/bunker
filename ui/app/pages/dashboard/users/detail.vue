<script setup lang="ts">
import type { FormError, FormSubmitEvent, TableColumn } from "@nuxt/ui";
import { guardWorking } from "~/composables/error";
import { fill, formatTime } from "~/utils/format";

const { $t } = useNuxtApp();

definePageMeta({
  middleware: ["auth", "admin"],
});

const route = useRoute();
const userId = (route.query.user_id as string) || "";

if (!userId) {
  await navigateTo({ name: "dashboard-users" });
}

const { data: grants, refresh: refreshGrants } = await useGrants(userId);

const columns: TableColumn<any>[] = [
  {
    accessorKey: "server_user",
    header: $t("common.server_user"),
  },
  {
    accessorKey: "server_id",
    header: $t("common.server_id"),
  },
  {
    id: "created_at",
    header: $t("common.created_at"),
  },
  {
    id: "actions",
  },
];

const state = reactive<{
  server_user?: string;
  server_id?: string;
}>({
  server_user: "*",
  server_id: "*",
});

const validate = (state: any): FormError[] => {
  const errors = [];
  if (!state.server_user) errors.push({ name: "server_user", message: $t("common.required") });
  if (!state.server_id) errors.push({ name: "server_id", message: $t("common.required") });
  return errors;
};

const working = ref(0);

async function onSubmit(event: FormSubmitEvent<any>) {
  await guardWorking(working, async () => {
    await $fetch("/backend/grants/create", {
      method: "POST",
      body: { user_id: userId, ...event.data },
    });

    await refreshGrants();
  });
}

async function deleteGrant({ id, server_user, server_id }: { id: string; server_user: string; server_id: string }) {
  if (!confirm(fill($t("grants.confirm_delete"), { target: `${server_user}@${server_id}` }))) {
    return;
  }

  await guardWorking(working, async () => {
    await $fetch("/backend/grants/delete", {
      method: "POST",
      body: { id },
    });

    await refreshGrants();
  });
}
</script>

<template>
  <SkeletonDashboard :title-name="$t('grants.title') + ' - ' + userId" title-icon="i-mdi-server-shield">
    <template #left>
      <UCard :ui="uiCard">
        <template #header>
          <div class="flex flex-row items-center">
            <UIcon name="i-mdi-server-plus" class="me-1" />
            <span>{{ $t("grants.add_grant") }}</span>
          </div>
        </template>
        <UForm :validate="validate" :state="state" class="space-y-4" @submit="onSubmit">
          <UFormField :label="$t('common.server_user')" name="server_user">
            <UInput v-model="state.server_user" />
          </UFormField>

          <UFormField :label="$t('common.server_id')" name="server_id">
            <UInput v-model="state.server_id" />
          </UFormField>

          <UButton
            type="submit"
            icon="i-mdi-check-circle"
            :label="$t('common.submit')"
            :loading="!!working"
            :disabled="!!working"
          />

          <p class="text-sm text-muted">{{ $t("grants.intro_asterisk") }}</p>
        </UForm>
      </UCard>
    </template>

    <UTable :data="grants.grants" :columns="columns">
      <template #created_at-cell="{ row }">
        {{ formatTime(row.original.created_at) }}
      </template>
      <template #actions-cell="{ row }">
        <UButton
          variant="link"
          color="error"
          icon="i-mdi-trash"
          :label="$t('common.delete')"
          :disabled="!!working"
          :loading="!!working"
          @click="deleteGrant(row.original)"
        />
      </template>
      <template #empty>
        <div class="py-6 text-center text-sm text-muted">{{ $t("common.empty") }}</div>
      </template>
    </UTable>
  </SkeletonDashboard>
</template>
