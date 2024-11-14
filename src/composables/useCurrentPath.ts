import { ref } from 'vue';

const currentPath = ref('/');

export function useCurrentPath() {
  const setPath = (path: string) => {
    currentPath.value = path;
  };

  return {
    currentPath,
    setPath
  };
}