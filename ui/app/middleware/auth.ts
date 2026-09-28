export default defineNuxtRouteMiddleware(async () => {
  const { data } = await useCurrentUser();
  const { $t } = useNuxtApp();

  if (!data.value.user) {
    const toast = useToast();

    toast.add({
      id: "not-signed-in",
      title: $t("auth.not_signed_in"),
      color: "error",
    });

    return navigateTo({ name: "index" });
  }
});
