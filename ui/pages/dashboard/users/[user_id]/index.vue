<script setup lang="ts">
import type { FormError, FormSubmitEvent } from "#ui/types";
import { guardWorking } from "~/composables/error";

definePageMeta({
    middleware: ["auth"],
})

const { data: grants, refresh: refreshGrants } = await useGrants(useRoute().params.user_id as string);

const columns = [
    {
        key: "server_user",
        label: 'Server User'
    },
    {
        key: 'server_id',
        label: 'Server ID'
    },
    {
        key: 'created_at',
        label: 'Created At'
    },
    {
        key: 'actions'
    }
];

const state = reactive<{
    server_user?: string;
    server_id?: string;
}>({
    server_user: '*',
    server_id: "*",
});

const validate = (state: any): FormError[] => {
    const errors = [];
    if (!state.server_user) errors.push({ path: "server_user", message: "Required" });
    if (!state.server_id) errors.push({ path: "server_id", message: "Required" });
    return errors;
};

const working = ref(0);

async function onSubmit(event: FormSubmitEvent<any>) {
    await guardWorking(working, async () => {

        await $fetch("/backend/grants/create", {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(Object.assign({ user_id: useRoute().params.user_id }, event.data))
        })

        await refreshGrants()

    })
}

async function deleteGrant({ id, server_user, server_id }: { id: string; server_user: string; server_id: string }) {
    if (!confirm(`confirm to to delete grant to ${server_user}@${server_id}?`)) {
        return
    }

    await guardWorking(working, async () => {

        await $fetch("/backend/grants/delete", {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ id })
        })

        await refreshGrants()

    })
}

</script>

<template>
    <SkeletonDashboard :title-name="'Grants: ' + $route.params.user_id" title-icon="i-mdi-server-shield">
        <template #left>
            <UCard :ui="uiCard">
                <template #header>
                    <div class="flex flex-row items-center">
                        <UIcon name="i-mdi-server-plus" class="me-1"></UIcon>
                        <span>Add Grant</span>
                    </div>
                </template>
                <UForm :validate="validate" :state="state" class="space-y-4" @submit="onSubmit">
                    <UFormGroup label="Server User" name="server_user">
                        <UInput v-model="state.server_user" />
                    </UFormGroup>

                    <UFormGroup label="Server ID" name="server_id">
                        <UInput v-model="state.server_id" type="password" />
                    </UFormGroup>

                    <UButton type="submit" icon="i-mdi-check-circle" label="Submit" :loading="!!working"
                        :disabled="!!working">
                    </UButton>
                </UForm>
            </UCard>
        </template>


        <UTable :rows="grants.grants" :columns="columns">
            <template #actions-data="{ row }">
                <UButton variant="link" color="red" icon="i-mdi-trash" label="Delete" @click="deleteGrant(row)"
                    :disabled="!!working" :loading="!!working"></UButton>
            </template>
        </UTable>

    </SkeletonDashboard>
</template>