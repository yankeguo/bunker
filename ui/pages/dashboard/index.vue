<script setup lang="ts">
const { $t } = useNuxtApp()

definePageMeta({
  middleware: ["auth"],
});

const { data: items } = await useGrantedItems();

const { data: uiOptions } = await useUIOptions();

function exampleUser(pattern: string) {
  if (pattern === "*") {
    return "root";
  }
  if (pattern.includes("*") || pattern.includes("?")) {
    return "<user>";
  }
  return pattern;
}

function sshCommand(row: { server_user: string; server_id: string }) {
  const host = uiOptions.value.ssh_host || "BUNKER_ADDRESS";
  const destination = `${exampleUser(row.server_user)}@${row.server_id}@${host}`;
  const port = uiOptions.value.ssh_port;
  if (port && port !== "22") {
    return `ssh -p ${port} ${destination}`;
  }
  return `ssh ${destination}`;
}

async function copyCommand(command: string) {
  try {
    await navigator.clipboard.writeText(command);
  } catch {
    const input = document.createElement("textarea");
    input.value = command;
    input.setAttribute("readonly", "true");
    document.body.appendChild(input);
    input.select();
    document.execCommand("copy");
    input.remove();
  }
  useToast().add({ title: $t("common.copied"), color: "green" });
}

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

</script>

<template>
  <SkeletonDashboard :title-name="$t('dashboard.title')" title-icon="i-mdi-view-dashboard">
    <template #left>
      <UCard :ui="uiCard">
        <article v-if="uiOptions.ssh_host" class="prose dark:prose-invert mb-4">
          <p>{{ $t('dashboard.ssh_address') }}: <span class="font-semibold">{{ uiOptions.ssh_host }}</span><span class="font-semibold"
              v-if="uiOptions.ssh_port">:{{
                uiOptions.ssh_port }}</span></p>
        </article>
        <article class="prose dark:prose-invert" v-html="$t('dashboard.intro')"></article>
      </UCard>
    </template>
    <UTable :rows="items.granted_items" :columns="columns">
      <template #example-data="{ row }">
        <div class="flex items-start gap-2">
          <code class="font-mono break-all">{{ sshCommand(row) }}</code>
          <UButton size="2xs" variant="ghost" color="gray" icon="i-mdi-content-copy" :aria-label="$t('dashboard.copy')"
            @click="copyCommand(sshCommand(row))" />
        </div>
      </template>
      <template #empty-state>
        <div class="py-6 text-center text-sm text-gray-500">{{ $t('common.empty') }}</div>
      </template>
    </UTable>
  </SkeletonDashboard>
</template>
