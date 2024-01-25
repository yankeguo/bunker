<script setup lang="ts">
const props = defineProps<{
  fields: Array<{
    name: string;
    content: string;
  }>;
}>();
</script>

<template>
  <template v-for="field in fields" v-bind:key="field.name">
    <div class="my-2">
      <label class="text-gray-900 dark:text-white font-semibold">{{
        field.name
      }}</label>
    </div>
    <div>
      <span class="text-gray-500 dark:text-gray-400">{{ field.content }}</span>
    </div>
  </template>
</template>
