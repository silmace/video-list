import { computed, ref } from 'vue';
import type { FileItem } from '../types';
import { getStoredJSON, setStoredString } from '@/lib/safeStorage';

export type FileType = 'folder' | 'video' | 'image' | 'audio' | 'archive' | 'document' | 'code' | 'other';

export interface CustomColorTag {
  id: string;
  label: string;
  pattern: string;
  color: string;
}

const TAG_STORAGE_KEY = 'video_list_color_tags';

const defaultTags: CustomColorTag[] = [
  { id: 'tag-video', label: 'Video', pattern: '.mp4,.mov,.mkv', color: '#F97316' },
  { id: 'tag-image', label: 'Image', pattern: '.jpg,.jpeg,.png,.webp', color: '#06B6D4' },
  { id: 'tag-work', label: 'Work', pattern: '.pdf,.doc,.docx', color: '#3B82F6' },
];

const fileTypePalette: Record<FileType, string> = {
  folder: '#EAB308',
  video: '#F97316',
  image: '#06B6D4',
  audio: '#14B8A6',
  archive: '#A855F7',
  document: '#3B82F6',
  code: '#22C55E',
  other: '#64748B',
};

const customTags = ref<CustomColorTag[]>(loadStoredTags());

function loadStoredTags(): CustomColorTag[] {
  const parsed = getStoredJSON<CustomColorTag[]>(TAG_STORAGE_KEY, [...defaultTags]);
  if (!Array.isArray(parsed) || parsed.length === 0) {
    return [...defaultTags];
  }
  return parsed.filter((item) =>
    typeof item.id === 'string' &&
    typeof item.label === 'string' &&
    typeof item.pattern === 'string' &&
    typeof item.color === 'string'
  );
}

function persistTags() {
  setStoredString(TAG_STORAGE_KEY, JSON.stringify(customTags.value));
}

function normalizedPatterns(pattern: string): string[] {
  return pattern
    .split(',')
    .map((item) => item.trim().toLowerCase())
    .filter(Boolean);
}

export function inferFileType(file: Pick<FileItem, 'name' | 'isDirectory'>): FileType {
  if (file.isDirectory) {
    return 'folder';
  }

  const name = file.name.toLowerCase();
  if (/\.(mp4|mov|mkv|avi|webm|m4v)$/.test(name)) {
    return 'video';
  }
  if (/\.(png|jpe?g|gif|bmp|webp|svg)$/.test(name)) {
    return 'image';
  }
  if (/\.(mp3|wav|flac|aac|m4a|ogg)$/.test(name)) {
    return 'audio';
  }
  if (/\.(zip|rar|7z|tar|gz|bz2)$/.test(name)) {
    return 'archive';
  }
  if (/\.(pdf|docx?|pptx?|xlsx?|txt|md)$/.test(name)) {
    return 'document';
  }
  if (/\.(go|ts|tsx|js|jsx|vue|json|yaml|yml|toml|py|java|rs|cpp|h)$/.test(name)) {
    return 'code';
  }
  return 'other';
}

export function getTypeAccent(fileType: FileType): string {
  return fileTypePalette[fileType];
}

export function getMatchingTag(file: Pick<FileItem, 'name' | 'path' | 'isDirectory'>): CustomColorTag | null {
  if (file.isDirectory) {
    return null;
  }

  const lowerName = file.name.toLowerCase();
  const lowerPath = file.path.toLowerCase();

  for (const tag of customTags.value) {
    const patterns = normalizedPatterns(tag.pattern);
    if (patterns.length === 0) {
      continue;
    }

    for (const pattern of patterns) {
      if (pattern.startsWith('.')) {
        if (lowerName.endsWith(pattern)) {
          return tag;
        }
      } else if (lowerName.includes(pattern) || lowerPath.includes(pattern)) {
        return tag;
      }
    }
  }

  return null;
}

export function getFileAccent(file: Pick<FileItem, 'name' | 'path' | 'isDirectory'>): string {
  const tag = getMatchingTag(file);
  if (tag) {
    return tag.color;
  }
  return getTypeAccent(inferFileType(file));
}

export function getDominantAccent(files: FileItem[]): string {
  if (files.length === 0) {
    return '#3B82F6';
  }

  const counters: Record<FileType, number> = {
    folder: 0,
    video: 0,
    image: 0,
    audio: 0,
    archive: 0,
    document: 0,
    code: 0,
    other: 0,
  };

  for (const file of files) {
    const type = inferFileType(file);
    counters[type] += file.isDirectory ? 2 : 1;
  }

  const [dominantType] = Object.entries(counters).sort((a, b) => b[1] - a[1])[0] as [FileType, number];
  return fileTypePalette[dominantType];
}

export function useFileVisuals() {
  const tagList = computed(() => customTags.value);

  const addTag = (tag: Omit<CustomColorTag, 'id'>) => {
    const label = tag.label.trim();
    const pattern = tag.pattern.trim();
    const color = tag.color.trim() || '#3B82F6';
    if (!label || !pattern) {
      return;
    }

    customTags.value = [
      ...customTags.value,
      {
        id: `tag-${Date.now()}-${Math.round(Math.random() * 1000)}`,
        label,
        pattern,
        color,
      },
    ];
    persistTags();
  };

  const removeTag = (id: string) => {
    customTags.value = customTags.value.filter((tag) => tag.id !== id);
    persistTags();
  };

  const resetTags = () => {
    customTags.value = [...defaultTags];
    persistTags();
  };

  return {
    tagList,
    addTag,
    removeTag,
    resetTags,
    inferFileType,
    getFileAccent,
    getTypeAccent,
    getMatchingTag,
    getDominantAccent,
  };
}
