export const useGrantedItems = () => {
    return useAsyncData<{ granted_items: BGrantedItem[] }>(
        "granted-items",
        () => $fetch("/backend/granted_items"),
        {
            default() {
                return { granted_items: [] };
            },
        }
    );
};
