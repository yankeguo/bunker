export const uiCard = {
    header: {
        padding: 'py-2 px-1'
    },
    body: {
        padding: 'py-1 px-1'
    },
    footer: {
        padding: 'py-2 px-1'
    }
}

export const useUIOptions = () => {
    return useAsyncData<{ ssh_host?: string; ssh_port?: number }>(
        "ui-options",
        () => $fetch("/backend/ui_options"),
        {
            default() {
                return { ssh_host: '', ssh_port: 0 };
            }
        }
    )
}