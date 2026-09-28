export const useHostKeys = () => {
  return useAsyncData<{ host_keys: BHostKey[] }>(
    "host-keys",
    () => $fetch("/backend/host_keys"),
    {
      default() {
        return { host_keys: [] };
      },
    }
  );
};
