export const useKeys = () => {
  return useAsyncData<{ keys: BKey[] }>("keys", () => $fetch("/backend/keys"), {
    default() {
      return { keys: [] };
    },
  });
};
