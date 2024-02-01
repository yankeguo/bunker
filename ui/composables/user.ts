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

export const useUsers = () => {
  return useAsyncData<{ users: BUser[] }>(
    "servers",
    () => $fetch("/backend/users"),
    {
      default() {
        return { users: [] };
      },
    }
  );
};
