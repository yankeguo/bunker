export const handleError = (e: any) => {
  const toast = useToast();
  toast.add({
    title: e.data?.message || "An error occurred",
    color: "red",
  });
};
