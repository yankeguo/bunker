<script setup lang="ts">
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
  document.cookie = `lang=${lang};path=/;max-age=31536000;samesite=lax`;
  location.reload();
}
</script>

<template>
  <UContainer
    class="fixed py-2 bottom-0 left-0 right-0 flex flex-row justify-between items-center bg-slate-100 dark:bg-slate-800"
  >
    <UButton
      to="https://github.com/yankeguo/bunker"
      target="_blank"
      variant="link"
      size="sm"
      color="neutral"
      icon="i-simple-icons-github"
      label="yankeguo/bunker"
    />

    <div class="flex flex-row items-center">
      <ClientOnly>
        <template v-for="item in $langs" :key="item">
          <a
            @click.prevent="setLanguage(item)"
            :class="{
              'text-sm': true,
              underline: $lang === item,
              'me-2': true,
            }"
            href="#"
          >
            <span>{{ $langNames[item] }}</span>
          </a>
          <span class="text-muted me-2">·</span>
        </template>

        <UButton
          :icon="isDark ? 'i-heroicons-moon-20-solid' : 'i-heroicons-sun-20-solid'"
          size="xs"
          color="neutral"
          variant="ghost"
          aria-label="Theme"
          square
          @click="isDark = !isDark"
        />

        <template #fallback>
          <div class="w-8 h-8"></div>
        </template>
      </ClientOnly>
    </div>
  </UContainer>
</template>
