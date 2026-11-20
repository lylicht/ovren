<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useVirtualizer } from '@tanstack/vue-virtual';
import { useI18n } from 'vue-i18n';

import MdIcon from '@/components/icons/md-icon.vue';
import { ICON_NAMES } from '@/lib/models/presentation/ui';
import type { FileBrowserEntry } from '@/lib/models/filesystem/file-browser';
import type { BrowserView } from '@/lib/utils/filesystem/file-browser';
import { browserEntryIcon } from '@/lib/utils/filesystem/file-browser';
import { ByteSizeService } from '@/lib/services/settings/byte-size-service';
import { FormatService } from '@/lib/services/presentation/format-service';

const props = defineProps<{
  entries: FileBrowserEntry[];
  selectedPaths: string[];
  view: BrowserView;
  busy: boolean;
  searchActive: boolean;
}>();

const emit = defineEmits<{
  select: [entry: FileBrowserEntry, event: MouseEvent];
  activate: [entry: FileBrowserEntry];
  context: [entry: FileBrowserEntry, event: MouseEvent];
  dragStart: [entry: FileBrowserEntry, event: DragEvent];
  dropOn: [entry: FileBrowserEntry, event: DragEvent];
}>();

const { t, locale } = useI18n({ useScope: 'global' });
const viewport = ref<HTMLElement | null>(null);
const viewportWidth = ref(750);
let resizeObserver: ResizeObserver | null = null;

const columns = computed(() => (props.view === 'grid' ? Math.max(1, Math.floor(viewportWidth.value / 178)) : 1));
const height = computed(() => (props.view === 'grid' ? 112 : 56));
const selected = computed(() => new Set(props.selectedPaths));
const virtualizer = useVirtualizer(
  computed(() => ({
    count: Math.ceil(props.entries.length / columns.value),
    getScrollElement: () => viewport.value,
    estimateSize: () => height.value,
    overscan: 10,
  }))
);
const visibleRows = computed(() =>
  virtualizer.value.getVirtualItems().map(row => ({
    row,
    items: props.entries
      .slice(row.index * columns.value, (row.index + 1) * columns.value)
      .map((entry, offset) => ({ entry, index: row.index * columns.value + offset })),
  }))
);

onMounted(() => {
  if (!viewport.value || typeof ResizeObserver === 'undefined') return;
  resizeObserver = new ResizeObserver(([entry]) => {
    if (entry) viewportWidth.value = entry.contentRect.width;
  });
  resizeObserver.observe(viewport.value);
});
onBeforeUnmount(() => resizeObserver?.disconnect());
watch(
  () => props.selectedPaths.at(-1),
  path => {
    if (!path) return;
    const index = props.entries.findIndex(entry => entry.path === path);
    if (index >= 0) virtualizer.value.scrollToIndex(Math.floor(index / columns.value), { align: 'auto' });
  }
);

function modified(entry: FileBrowserEntry) {
  return entry.modifiedAtMs === null ? '—' : FormatService.dateTime(entry.modifiedAtMs, locale.value);
}
</script>

<template>
  <div ref="viewport" class="browser-viewport scrollbar-stable" :class="{ 'as-grid': view === 'grid' }">
    <div
      class="browser-virtual-space"
      role="listbox"
      aria-multiselectable="true"
      :aria-label="t('navigation.file-browser')"
      :style="{ height: virtualizer.getTotalSize() + 'px' }"
    >
      <div
        v-for="group in visibleRows"
        :key="group.row.index"
        class="browser-virtual-row"
        :style="{
          height: height + 'px',
          transform: 'translateY(' + group.row.start + 'px)',
          gridTemplateColumns: 'repeat(' + columns + ', minmax(0, 1fr))',
        }"
      >
        <button
          v-for="{ entry } in group.items"
          :key="entry.path"
          type="button"
          role="option"
          :draggable="!busy"
          class="browser-entry"
          :class="{ selected: selected.has(entry.path), 'grid-entry': view === 'grid' }"
          :aria-selected="selected.has(entry.path)"
          :aria-label="entry.name"
          :disabled="busy"
          :data-entry-path="entry.path"
          @click="emit('select', entry, $event)"
          @dblclick="emit('activate', entry)"
          @contextmenu.prevent="emit('context', entry, $event)"
          @dragstart="emit('dragStart', entry, $event)"
          @dragover="entry.isDirectory && !entry.isSymlink && $event.preventDefault()"
          @drop.prevent="emit('dropOn', entry, $event)"
        >
          <span class="entry-icon">
            <MdIcon :name="browserEntryIcon(entry)" :size="view === 'grid' ? 31 : 20" />
          </span>
          <span class="entry-name" dir="auto">{{ entry.name }}</span>
          <template v-if="view === 'details'">
            <span class="entry-size">{{ entry.isDirectory ? '—' : ByteSizeService.bytes(entry.size) }}</span>
            <span class="entry-modified">{{ modified(entry) }}</span>
          </template>
          <span v-else class="entry-subtitle">
            {{ entry.isDirectory ? t('fileBrowser.folderKind') : ByteSizeService.bytes(entry.size) }}
          </span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "@assets/main.css";
.browser-viewport {
  min-height: 0;
  flex: 1;
  overflow: auto;
}
.browser-virtual-space {
  position: relative;
  width: 100%;
}
.browser-virtual-row {
  position: absolute;
  inset: 0 0 auto;
  display: grid;
  width: 100%;
}
.browser-entry {
  display: grid;
  min-width: 0;
  grid-template-columns: 30px minmax(0, 1fr) 105px 160px;
  align-items: center;
  gap: 10px;
  border: 0;
  border-bottom: 1px solid var(--border);
  padding: 4px 12px;
  text-align: start;
  cursor: default;
  font-size: var(--font-content-body);
  @apply bg-transparent text-foreground;
}
.browser-entry:hover {
  @apply bg-muted/60;
}
.browser-entry:focus-visible {
  outline: 2px solid var(--ring);
  outline-offset: -2px;
}
.browser-entry.selected {
  @apply bg-primary/15;
}
.entry-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  @apply text-primary-text;
}
.entry-name,
.entry-size,
.entry-modified,
.entry-subtitle {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.entry-size,
.entry-modified,
.entry-subtitle {
  font-size: var(--font-content-secondary);
  @apply text-muted-foreground;
}
.entry-size,
.entry-modified {
  text-align: end;
}
.as-grid .browser-virtual-row {
  gap: 0;
}
.browser-entry.grid-entry {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 5px;
  border: 1px solid transparent;
  border-radius: 9px;
  padding: 8px 6px;
  text-align: center;
}
.browser-entry.grid-entry.selected {
  border-color: var(--border-primary-subtle);
}
.grid-entry .entry-name {
  max-width: 100%;
  font-weight: 500;
}
.grid-entry .entry-subtitle {
  max-width: 100%;
  font-size: var(--font-content-meta);
}
@container file-browser (max-width: 740px) {
  .browser-entry:not(.grid-entry) {
    grid-template-columns: 28px minmax(0, 1fr) 96px;
  }
  .browser-entry:not(.grid-entry) .entry-modified {
    display: none;
  }
}
@container file-browser (max-width: 460px) {
  .browser-entry:not(.grid-entry) {
    grid-template-columns: 28px minmax(0, 1fr);
  }
  .browser-entry:not(.grid-entry) .entry-size {
    display: none;
  }
}
</style>
