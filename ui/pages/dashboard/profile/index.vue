<script setup lang="ts">
import type { FormError, FormSubmitEvent } from "#ui/types";
import { guardWorking } from "~/composables/error";

definePageMeta({
  middleware: ["auth"],
});

const { $t } = useNuxtApp()

const { data: currentUser } = await useCurrentUser();

const fields = computed(() => [
  {
    name: $t('common.user_id'),
    content: currentUser.value.user?.id || "",
  },
  {
    name: $t('common.user_role'),
    content: currentUser.value.user?.is_admin ? $t('common.user_role_admin') : $t('common.user_role_standard'),
  },
  {
    name: $t('common.created_at'),
    content: currentUser.value.user?.created_at || "",
  },
]);

async function doSignOut() {
  if (!confirm("Are you sure to sign out?")) {
    return;
  }
  await $fetch("/backend/sign_out");
  navigateTo({ name: "index" });
}

const state = reactive<{
  old_password?: string;
  new_password?: string;
  repeat_password?: string;
}>({
  old_password: undefined,
  new_password: undefined,
  repeat_password: undefined,
});

const validate = (state: any): FormError[] => {
  const errors = [];
  if (!state.old_password) errors.push({ path: "old_password", message: "Required" });
  if (!state.new_password) errors.push({ path: "new_password", message: "Required" });
  if (!state.repeat_password) errors.push({ path: "repeat_password", message: "Required" });
  if (state.new_password && state.new_password.length < 6) errors.push({ path: "new_password", message: "Too short, must >= 6" });
  if (state.new_password !== state.repeat_password) errors.push({ path: "repeat_password", message: "Not match" });
  return errors;
};

const working = ref(0);

async function onSubmit(event: FormSubmitEvent<any>) {
  await guardWorking(working, async () => {
    await $fetch("/backend/update_password", {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(event.data)
    })
    state.old_password = ''
    state.new_password = ''
    state.repeat_password = ''
    const toast = useToast()
    toast.add({ title: $t('profile.password_updated'), color: 'green' })
  })
}
</script>

<template>
  <SkeletonDashboard :title-name="$t('profile.title')" title-icon="i-mdi-account-circle">
    <template #left>
      <UCard :ui="uiCard">
        <template #header>
          <div class="flex flex-row items-center">
            <UIcon name="i-mdi-account-circle" class="me-1"></UIcon>
            <span>{{ $t('profile.title') }}</span>
          </div>
        </template>
        <SimpleFields :fields="fields"></SimpleFields>
        <div class="mt-6">
          <UButton icon="i-mdi-logout" :label="$t('common.sign_out')" color="red" @click="doSignOut"></UButton>
        </div>
      </UCard>
    </template>
    <UCard :ui="uiCard" class="w-80">
      <template #header>
        <div class="flex flex-row items-center">
          <UIcon name="i-mdi-form-textbox-password" class="me-1"></UIcon>
          <span>{{ $t('profile.update_password') }}</span>
        </div>
      </template>
      <UForm :validate="validate" :state="state" class="space-y-4" @submit="onSubmit">
        <UFormGroup :label="$t('profile.old_password')" name="old_password">
          <UInput v-model="state.old_password" :placeholder="$t('profile.input_old_password')" type="password" />
        </UFormGroup>

        <UFormGroup :label="$t('profile.new_password')" name="new_password">
          <UInput v-model="state.new_password" type="password" :placeholder="$t('profile.input_new_password')" />
        </UFormGroup>


        <UFormGroup :label="$t('profile.repeat_password')" name="repeat_password">
          <UInput v-model="state.repeat_password" type="password" :placeholder="$t('profile.input_repeat_password')" />
        </UFormGroup>

        <UButton type="submit" icon="i-mdi-check-circle" :label="$t('common.submit')" :loading="!!working"
          :disabled="!!working">
        </UButton>
      </UForm>

    </UCard>
  </SkeletonDashboard>
</template>
