export const handleError = (e: any) => {
  const toast = useToast();
  toast.add({
    title: e.data?.message || "An error occurred",
    color: "red",
  });
};

export async function guardWorking<T>(counter: Ref<number>, fn: () => Promise<T>) {
  counter.value++

  try {
    return await fn()
  } catch (e: any) {
    useToast().add({
      title: e.data?.message || "An error occurred",
      color: "red",
    });
  } finally {
    counter.value--
  }

}