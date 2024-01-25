<script setup lang="ts">
import type { FormError, FormSubmitEvent } from "#ui/types";

definePageMeta({
  middleware: ["auth"],
});

const state = reactive({
  display_name: undefined,
  public_key: undefined,
});

const validate = (state: any): FormError[] => {
  const errors = [];
  if (!state.display_name)
    errors.push({ path: "display_name", message: "Required" });
  if (!state.public_key)
    errors.push({ path: "public_key", message: "Required" });
  return errors;
};

const working = ref(false);

async function onSubmit(event: FormSubmitEvent<any>) {
  working.value = true;

  try {
    await $fetch("/backend/keys/create", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(event.data),
    });
    navigateTo({ name: "dashboard-profile-keys" });
  } catch (e: any) {
    handleError(e);
  } finally {
    working.value = false;
  }
}
</script>

<template>
  <SkeletonDashboard title-name="SSH Keys" title-icon="i-mdi-key-chain">
    <UForm
      :validate="validate"
      :state="state"
      class="space-y-4 w-80"
      @submit="onSubmit"
    >
      <UFormGroup label="Display Name" name="display_name">
        <UInput v-model="state.display_name" />
      </UFormGroup>

      <UFormGroup label="Public Key" name="public_key">
        <UTextarea v-model="state.public_key" :rows="12" />
      </UFormGroup>

      <UButton
        type="submit"
        :disabled="working"
        :loading="working"
        icon="i-mdi-check"
        label="Submit"
      >
      </UButton>
    </UForm>
  </SkeletonDashboard>
</template>
