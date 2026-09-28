export const useServers = () => {
  return useAsyncData<{ servers: BServer[] }>(
    "servers",
    () => $fetch("/backend/servers"),
    {
      default() {
        return { servers: [] };
      },
    }
  );
};
