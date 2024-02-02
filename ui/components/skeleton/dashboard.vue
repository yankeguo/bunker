<script setup lang="ts">
const { $t } = useNuxtApp()
const props = defineProps<{
  titleName: string;
  titleIcon: string;
}>();

const route = useRoute();

const { data: user } = await useCurrentUser();

const links = [
  [
    {
      label: $t("dashboard.title"),
      icon: "i-mdi-view-dashboard",
      to: { name: "dashboard" },
    },
    ...(user.value.user?.is_admin
      ? [
        {
          label: $t("servers.title"),
          icon: "i-mdi-server",
          to: { name: "dashboard-servers" },
        },
        {
          label: "Users",
          icon: "i-mdi-account-multiple",
          to: { name: "dashboard-users" },
        },
      ]
      : []),
  ],
  [
    {
      label: "SSH Keys",
      icon: "i-mdi-key-chain",
      to: { name: "dashboard-profile-keys" },
    },
    {
      label: "Profile",
      icon: "i-mdi-account-circle",
      to: { name: "dashboard-profile" },
    },
  ],
];
</script>

<template>
  <div class="flex flex-col my-6">

    <Head>
      <Title>Bunker - {{ titleName }}</Title>
    </Head>

    <UHorizontalNavigation :links="links" class="w-full border-b border-gray-200 dark:border-gray-800" />

    <div class="flex flex-row items-center mt-8 px-2.5">
      <UIcon :name="titleIcon" class="text-2xl font-semibold me-2" size="lg"></UIcon>
      <span class="text-2xl font-semibold">{{ titleName }}</span>
    </div>

    <div class="flex flex-row mt-8 px-2.5">
      <div class="w-80 me-8">
        <slot name="left"></slot>
      </div>
      <div class="flex-grow">
        <slot></slot>
      </div>
    </div>
  </div>
</template>
