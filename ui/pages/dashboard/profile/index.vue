<script setup lang="ts">
import { formatTimeAgo } from "@vueuse/core";
definePageMeta({
  middleware: ["auth"],
});

const { data: currentUser } = await useCurrentUser();

const fields = computed(() => [
  {
    name: "Username",
    content: currentUser.value.user?.id || "",
  },
  {
    name: "Is Admin",
    content: currentUser.value.user?.is_admin ? "Yes" : "No",
  },
  {
    name: "Created At",
    content: formatTimeAgo(new Date(currentUser.value.user?.created_at || "")),
  },
]);

async function doSignOut() {
  if (!confirm("Are you sure to sign out?")) {
    return;
  }
  await $fetch("/backend/sign_out");
  navigateTo({ name: "index" });
}
</script>

<template>
  <SkeletonDashboard title-name="Profile" title-icon="i-mdi-account-circle">
    <SimpleFields :fields="fields"></SimpleFields>
    <div class="mt-6">
      <UButton
        icon="i-mdi-logout"
        label="Sign out"
        color="red"
        @click="doSignOut"
      ></UButton>
    </div>
  </SkeletonDashboard>
</template>
