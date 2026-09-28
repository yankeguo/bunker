<template>
  <NuxtLoadingIndicator color="#0369a1" />
  <UContainer class="pb-32">
    <NuxtLayout>
      <NuxtPage></NuxtPage>
    </NuxtLayout>
  </UContainer>
  <UNotifications :ui="{ strategy: 'override', position: 'top-0 end-0' }" />
  <Footer></Footer>
</template>
