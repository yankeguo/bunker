<script setup lang="ts">
defineProps<{
  fields: Array<{
    name: string;
    content: string;
  }>;
}>();
</script>

<template>
  <template v-for="field in fields" v-bind:key="field.name">
    <div class="mb-1">
      <label class="text-highlighted font-semibold">{{ field.name }}</label>
    </div>
    <div class="mb-2">
      <span class="text-muted">{{ field.content }}</span>
    </div>
  </template>
</template>
