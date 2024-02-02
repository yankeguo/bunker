<script setup lang="ts">
import type { FormError, FormSubmitEvent } from "#ui/types";
import { guardWorking } from "~/composables/error";

const state = reactive({
  username: undefined,
  password: undefined,
});

const validate = (state: any): FormError[] => {
  const errors = [];
  if (!state.username) errors.push({ path: "username", message: "Required" });
  if (!state.password) errors.push({ path: "password", message: "Required" });
  return errors;
};

const working = ref(0);

async function onSubmit(event: FormSubmitEvent<any>) {
  return guardWorking(working, async () => {
    await $fetch("/backend/sign_in", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(event.data),
    });
    await refreshCurrentUser();
    navigateTo({ name: "dashboard" });
  })
}

const { data: currentUser, refresh: refreshCurrentUser } =
  await useCurrentUser();

if (currentUser.value.user && currentUser.value.token) {
  navigateTo({ name: "dashboard" });
}
</script>

<template>
  <div class="absolute top-0 left-0 w-full h-full flex flex-col justify-center items-center">
    <div class="mb-12 text-center">
      <div class="font-semibold text-4xl mb-6">Bunker System</div>
    </div>

    <UCard class="w-80">
      <UForm :validate="validate" :state="state" class="space-y-4" @submit="onSubmit">
        <UFormGroup :label="$t('common.username')" name="username">
          <UInput v-model="state.username" />
        </UFormGroup>

        <UFormGroup :label="$t('common.password')" name="password">
          <UInput v-model="state.password" type="password" />
        </UFormGroup>

        <UButton type="submit" icon="i-mdi-login" :disabled="!!working" :loading="!!working"
          :label="$t('common.sign_in')"></UButton>
      </UForm>
    </UCard>
  </div>
</template>
