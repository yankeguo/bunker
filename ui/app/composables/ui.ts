export const uiCard = {
  header: "py-2 px-3",
  body: "py-2 px-3",
  footer: "py-2 px-3",
};

export const useUIOptions = () => {
  return useAsyncData<{ ssh_host?: string; ssh_port?: string }>(
    "ui-options",
    () => $fetch("/backend/ui_options"),
    {
      default() {
        return { ssh_host: "", ssh_port: "" };
      },
    },
  );
};
