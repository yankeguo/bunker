export const useCurrentUser = async () => {
  return useAsyncData<{
    user?: BUser;
    token?: BToken;
  }>("current-user", () => $fetch("/backend/current_user"), {
    default() {
      return {
        user: undefined,
        token: undefined,
      };
    },
  });
};
