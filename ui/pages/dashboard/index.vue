<script setup lang="ts">
import type { FormError, FormSubmitEvent } from "#ui/types";
import { guardWorking } from "~/composables/error";

const { $t } = useNuxtApp()

definePageMeta({
  middleware: ["auth"],
});

const { data: items, refresh: refreshItems } = await useGrantedItems();

const columns = [
  {
    key: "server_user",
    label: $t('common.server_user'),
  },
  {
    key: "server_id",
    label: $t('common.server_id'),
  },
  {
    key: 'example',
    label: $t('dashboard.command_example')
  }
];

function expandServerUser(s: string): string {
  if (s === '*') {
    return 'root'
  }
  return s
}

</script>
<template>
  <SkeletonDashboard :title-name="$t('dashboard.title')" title-icon="i-mdi-view-dashboard">
    <template #left>
      <UCard :ui="uiCard">
        <article class="prose dark:prose-invert" v-html="$t('dashboard.intro')"></article>
      </UCard>
    </template>
    <UTable :rows="items.granted_items" :columns="columns">
      <template #example-data="{ row }">
        <code class="font-mono">ssh {{ expandServerUser(row.server_user) }}@{{ row.server_id }}@BUNKER_ADDRESS</code>
      </template>
    </UTable>
  </SkeletonDashboard>
</template>
