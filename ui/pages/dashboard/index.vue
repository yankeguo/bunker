<script setup lang="ts">
import type { FormError, FormSubmitEvent } from "#ui/types";
import { guardWorking } from "~/composables/error";

const { $t } = useNuxtApp()

definePageMeta({
  middleware: ["auth"],
});

const { data: items, refresh: refreshItems } = await useGrantedItems();

const { data: uiOptions, refresh: refreshUIOptions } = await useUIOptions();

const addressHint = computed(() => {
  if (uiOptions.value.ssh_host) {
    if (uiOptions.value.ssh_port) {
      return `${uiOptions.value.ssh_host} -p ${uiOptions.value.ssh_port}`
    } else {
      return uiOptions.value.ssh_host
    }
  }
  return 'BUNKER_ADDRESS'
})

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
        <article v-if="uiOptions.ssh_host" class="prose dark:prose-invert mb-4">
          <p>Bunker SSH 地址: <span class="font-semibold">{{ uiOptions.ssh_host }}</span><span class="font-semibold"
              v-if="uiOptions.ssh_port">:{{
                uiOptions.ssh_port }}</span></p>
        </article>
        <article class="prose dark:prose-invert" v-html="$t('dashboard.intro')"></article>
      </UCard>
    </template>
    <UTable :rows="items.granted_items" :columns="columns">
      <template #example-data="{ row }">
        <code class="font-mono">ssh {{ expandServerUser(row.server_user) }}@{{ row.server_id }}@{{ addressHint }}</code>
      </template>
    </UTable>
  </SkeletonDashboard>
</template>
