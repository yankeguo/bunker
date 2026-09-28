<template>
  <UApp :toaster="{ position: 'top-right', class: 'top-20' }">
    <NuxtLoadingIndicator color="#0369a1" />
    <UContainer class="pb-32">
      <NuxtLayout>
        <NuxtPage />
      </NuxtLayout>
    </UContainer>
    <Footer />
  </UApp>
</template>
