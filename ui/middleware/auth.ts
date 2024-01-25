export default defineNuxtRouteMiddleware(async (to, from) => {
  const { data } = await useCurrentUser();

  if (!data.value.user) {
    const toast = useToast();

    toast.add({
      id: "not-signed-in",
      title: "You are not signed in",
      color: "red",
    });

    return navigateTo({ name: "index" });
  }
});
