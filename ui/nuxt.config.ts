// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  devtools: { enabled: true },
  ssr: false,
  modules: ["@nuxt/ui", "@vueuse/nuxt"],
  plugins: ["~/plugins/i18n"],
  ui: {
    icons: ["heroicons", "simple-icons", "mdi", "noto-v1"],
  },
  colorMode: {
    preference: "dark",
  },
});
