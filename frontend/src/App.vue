<script setup lang="ts">
import { RouterView, useRoute } from 'vue-router';
import { useLiquidGlass } from 'latere-ui';

// v2 Liquid Glass lensing: SVG refraction + cursor sheen on glass chrome,
// re-scanned on navigation. Progressive enhancement (Chromium-only, honors
// reduced-motion / reduced-transparency); onMounted is client-only under SSG.
const route = useRoute();
useLiquidGlass({ watchSource: () => route.fullPath });
</script>

<template>
  <RouterView />
</template>
