<script setup lang="ts">
const props = defineProps<{
  titleName: string;
  titleIcon: string;
}>();

const route = useRoute();

const links = [
  [
    {
      label: "Dashboard",
      icon: "i-mdi-view-dashboard",
      to: { name: "dashboard" },
    },
    {
      label: "Servers",
      icon: "i-mdi-server",
      to: { name: "dashboard-servers" },
    },
    {
      label: "Users",
      icon: "i-mdi-account-multiple",
      to: { name: "dashboard-users" },
    },
  ],
  [
    {
      label: "Profile",
      icon: "i-mdi-account-circle",
      to: { name: "dashboard-profile" },
    },
  ],
];
</script>

<template>
  <div class="flex flex-col md:flex-row my-6">
    <Head>
      <Title>Bunker - {{ titleName }}</Title>
    </Head>

    <div
      class="w-full md:w-1/3 lg:w-1/5 px-3 py-6 flex flex-col items-center md:items-end"
    >
      <UVerticalNavigation
        class="w-full"
        :ui="{ size: 'text-lg' }"
        :links="links"
      />
    </div>

    <div class="w-full md:w-2/3 lg:w-4/5 px-12 py-6">
      <div class="flex flex-row items-center">
        <UIcon
          :name="titleIcon"
          class="text-2xl font-semibold me-2"
          size="lg"
        ></UIcon>
        <span class="text-2xl font-semibold">{{ titleName }}</span>
      </div>
      <div class="flex flex-col mt-8 px-1">
        <slot></slot>
      </div>
    </div>
  </div>
</template>
