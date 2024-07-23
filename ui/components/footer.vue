<script setup lang="ts">
const year = new Date().getFullYear();

const colorMode = useColorMode();

const isDark = computed({
  get() {
    return colorMode.value === "dark";
  },
  set() {
    colorMode.preference = colorMode.value === "dark" ? "light" : "dark";
  },
});

function setLanguage(lang: string) {
  document.cookie = `lang=${lang};path=/;max-age=31536000`;
  location.reload();
}
</script>

<template>
  <UContainer
    class="fixed py-2 bottom-0 left-0 right-0 flex flex-row justify-between items-center bg-slate-100 dark:bg-slate-800">
    <UButton to="https://github.com/yankeguo/bunker" target="_blank" variant="link" size="sm" color="black"
      icon="i-simple-icons-github" label="yankeguo/bunker"></UButton>

    <div class="flex flex-row items-center">
      <ClientOnly>
        <!-- i18n -->
        <template v-for="(item, idx) in $langs">
          <a @click.prevent="setLanguage(item)" :class="{
            'text-sm': true,
            underline: $lang === item,
            'me-2': true,
          }" href="#">
            <span>{{ $langNames[item] }}</span>
          </a>
          <i class="i-bi-dot text-slate-400" />
        </template>

        <UButton :icon="isDark ? 'i-heroicons-moon-20-solid' : 'i-heroicons-sun-20-solid'
          " size="2xs" color="black" variant="ghost" aria-label="Theme" :padded="false" @click="isDark = !isDark" />

        <template #fallback>
          <div class="w-8 h-8"></div>
        </template>
      </ClientOnly>
    </div>

  </UContainer>
</template>
