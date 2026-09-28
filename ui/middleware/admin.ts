export default defineNuxtRouteMiddleware(async () => {
  const { data } = await useCurrentUser();
  const { $t } = useNuxtApp();

  if (!data.value.user?.is_admin) {
    const toast = useToast();
    toast.add({
      id: "not-admin",
      title: $t("auth.not_admin"),
      color: "red",
    });
    return navigateTo({ name: "dashboard" });
  }
});
