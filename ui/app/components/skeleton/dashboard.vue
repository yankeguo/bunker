<script setup lang="ts">
import type { NavigationMenuItem } from "@nuxt/ui";

const { $t } = useNuxtApp();
defineProps<{
  titleName: string;
  titleIcon: string;
}>();

const { data: user } = await useCurrentUser();

const links: NavigationMenuItem[][] = [
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
            label: $t("users.title"),
            icon: "i-mdi-account-multiple",
            to: { name: "dashboard-users" },
          },
        ]
      : []),
  ],
  [
    {
      label: $t("ssh_keys.title"),
      icon: "i-mdi-key-chain",
      to: { name: "dashboard-profile-keys" },
    },
    {
      label: $t("profile.title"),
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

    <UNavigationMenu
      :items="links"
      class="w-full border-b border-default"
    />

    <div class="flex flex-row items-center mt-8 px-2.5">
      <UIcon :name="titleIcon" class="size-6 me-2" />
      <span class="text-2xl font-semibold">{{ titleName }}</span>
    </div>

    <div class="flex flex-col lg:flex-row mt-8 px-2.5 gap-8">
      <div class="w-full lg:w-80 shrink-0">
        <slot name="left"></slot>
      </div>
      <div class="flex-grow min-w-0 overflow-x-auto">
        <slot></slot>
      </div>
    </div>
  </div>
</template>
