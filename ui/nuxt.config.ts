// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  devtools: { enabled: true },
  ssr: false,
  modules: ["@nuxt/ui"],
  css: ["~/assets/css/main.css"],

  colorMode: {
    preference: "dark",
  },

  icon: {
    clientBundle: {
      scan: true,
    },
  },

  compatibilityDate: "2026-09-28",
});
