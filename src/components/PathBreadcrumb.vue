<script setup lang="ts">
import { computed } from 'vue';
import { useRouter } from 'vue-router';

const props = defineProps<{
  path: string;
  onNavigate?: (path: string) => void;
}>();

const router = useRouter();

const pathSegments = computed(() => {
  const segments = props.path.split('/').filter(Boolean);
  return [
    { name: 'Home', path: '/' },
    ...segments.map((segment, index) => ({
      name: segment,
      path: '/' + segments.slice(0, index + 1).join('/') + '/'
    }))
  ];
});

const handleClick = (path: string) => {
  if (props.onNavigate) {
    props.onNavigate(path);
  } else {
    router.push('/');
  }
};
</script>

<template>
  <div class="d-flex align-center">
    <span class="mr-2">Path:</span>
    <template v-for="(segment, index) in pathSegments" :key="segment.path">
      <v-btn
        variant="text"
        density="comfortable"
        @click="handleClick(segment.path)"
      >
        {{ segment.name }}
      </v-btn>
      <span v-if="index < pathSegments.length - 1">/</span>
    </template>
  </div>
</template>