<script setup lang="ts">
import type { FormError, FormSubmitEvent } from "#ui/types";

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

async function onSubmit(event: FormSubmitEvent<any>) {
  // Do something with data
  console.log(event.data);
}
</script>

<template>
  <div
    class="absolute top-0 left-0 w-full h-full flex flex-col justify-center items-center"
  >
    <div class="mb-12">
      <div class="font-semibold font-mono text-4xl mb-4">Bunker System</div>
      <div class="flex flex-row items-center">
        <span class="me-2">by</span>
        <UButton
          size="sm"
          icon="i-simple-icons-github"
          variant="link"
          to="https://github.com/yankeguo/bunker"
          target="_blank"
          label="yankeguo"
        ></UButton>
      </div>
    </div>

    <UCard class="w-80">
      <UForm
        :validate="validate"
        :state="state"
        class="space-y-4"
        @submit="onSubmit"
      >
        <UFormGroup label="Username" name="username">
          <UInput v-model="state.username" />
        </UFormGroup>

        <UFormGroup label="Password" name="password">
          <UInput v-model="state.password" type="password" />
        </UFormGroup>

        <UButton type="submit">Sign In</UButton>
      </UForm>
    </UCard>

    <div class="h-64"></div>
  </div>
</template>
