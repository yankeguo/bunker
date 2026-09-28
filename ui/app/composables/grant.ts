export const useGrants = (userId: string) => {
    return useAsyncData<{ grants: BGrant[] }>(
        "grants-" + userId,
        () => $fetch("/backend/grants", {
            query: {
                user_id: userId,
            }
        }),
        {
            default() {
                return { grants: [] };
            },
        }
    );
};
