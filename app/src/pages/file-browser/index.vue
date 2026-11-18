<script setup lang="ts">
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import MdEmptyState from '@/components/custom/md-empty-state.vue';
import MdPageShell from '@/components/custom/md-page-shell.vue';
import MdResultSearch from '@/components/custom/md-result-search.vue';
import MdResultWorkspace from '@/components/custom/md-result-workspace.vue';
import MdIcon from '@/components/icons/md-icon.vue';
import { Button } from '@/components/ui/button';
import type { FileBrowserEntry, FileBrowserSearchResult } from '@/lib/models/filesystem/file-browser';
import { ICON_NAMES } from '@/lib/models/presentation/ui';
import { FolderSelectionService } from '@/lib/services/storage/folder-selection-service';
import { FormatService } from '@/lib/services/presentation/format-service';
import { ByteSizeService } from '@/lib/services/settings/byte-size-service';
import { FileManagerService } from '@/lib/services/filesystem/file-manager-service';

const { locale, t } = useI18n({ useScope: 'global' });
const path = ref('');
const entries = ref<FileBrowserEntry[]>([]);
const query = ref('');
const searchResult = ref<FileBrowserSearchResult | null>(null);
const busy = ref(false);
const errorMessage = ref('');
let operation = 0;

const visibleEntries = computed(() => searchResult.value?.entries ?? entries.value);

async function chooseFolder() {
  if (busy.value) return;
  try {
    const [selected] = await FolderSelectionService.select(false, t('scanScope.chooseFolder'), path.value || undefined);
    if (selected) await loadDirectory(selected);
  } catch {
    errorMessage.value = t('common.operationFailed');
  }
}

async function loadDirectory(target: string) {
  if (busy.value) return;
  const request = ++operation;
  busy.value = true;
  errorMessage.value = '';
  try {
    const result = await FileManagerService.listBrowserDirectory(target);
    if (request !== operation) return;
    path.value = result.path;
    entries.value = result.entries;
    query.value = '';
    searchResult.value = null;
  } catch {
    errorMessage.value = t('common.operationFailed');
  } finally {
    if (request === operation) busy.value = false;
  }
}

async function search() {
  const text = query.value.trim();
  if (!path.value || !text || busy.value) return;
  const request = ++operation;
  busy.value = true;
  errorMessage.value = '';
  try {
    const result = await FileManagerService.searchBrowserDirectory(path.value, text);
    if (request === operation) searchResult.value = result;
  } catch {
    errorMessage.value = t('common.operationFailed');
  } finally {
    if (request === operation) busy.value = false;
  }
}

async function refresh() {
  if (searchResult.value) {
    await search();
    return;
  }
  if (path.value) await loadDirectory(path.value);
}

async function goUp() {
  if (!path.value || busy.value) return;
  try {
    const parent = await FileManagerService.parentDirectory(path.value);
    if (parent !== path.value) await loadDirectory(parent);
  } catch {
    errorMessage.value = t('common.operationFailed');
  }
}

async function activate(entry: FileBrowserEntry) {
  if (entry.isDirectory && !entry.isSymlink) {
    await loadDirectory(entry.path);
    return;
  }
  try {
    await FileManagerService.openBrowserEntry(entry.path);
  } catch {
    errorMessage.value = t('common.operationFailed');
  }
}
</script>

<template>
  <MdPageShell
    class="@container/file-browser"
    content-mode="workspace"
    content-width="wide"
    :title="t('navigation.file-browser')"
    :subtitle="path || undefined"
  >
    <template #actions>
      <Button data-testid="choose-folder" variant="outline" :disabled="busy" @click="chooseFolder">
        <MdIcon :name="ICON_NAMES.folderOpen" :size="16" />
        {{ t('scanScope.chooseFolder') }}
      </Button>
    </template>

    <MdResultWorkspace v-if="path" class="file-browser-workspace">
      <form class="file-browser-toolbar" @submit.prevent="search">
        <div class="file-browser-navigation">
          <Button
            variant="outline"
            type="button"
            :aria-label="t('fileBrowser.goUp')"
            :disabled="busy || !path"
            @click="goUp"
          >
            <MdIcon :name="ICON_NAMES.chevronUp" :size="16" />
            {{ t('fileBrowser.goUp') }}
          </Button>
          <Button
            variant="outline"
            type="button"
            :aria-label="t('monitoring.refresh')"
            :disabled="busy"
            @click="refresh"
          >
            <MdIcon :name="ICON_NAMES.refresh" :size="16" />
            {{ t('monitoring.refresh') }}
          </Button>
        </div>
        <MdResultSearch v-model="query" :placeholder="t('fileBrowser.search')" :disabled="busy" />
        <Button type="submit" :disabled="busy || !query.trim()">
          <MdIcon :name="ICON_NAMES.search" :size="16" />
          {{ t('fileBrowser.search') }}
        </Button>
      </form>

      <div class="file-browser-path" dir="auto">{{ path }}</div>

      <p v-if="searchResult" class="file-browser-summary" aria-live="polite">
        {{ t('common.itemCount', { count: searchResult.entries.length }) }}
      </p>
      <p v-if="searchResult?.truncated" class="file-browser-notice" role="status">
        {{ t('fileBrowser.resultsLimited') }}
      </p>
      <p v-if="searchResult?.skippedCount" class="file-browser-notice" role="status">
        {{ t('fileBrowser.skippedItems', { count: searchResult.skippedCount }) }}
      </p>
      <p v-if="errorMessage" class="file-browser-error" role="alert">{{ errorMessage }}</p>

      <div v-if="busy" class="file-browser-loading" role="status">
        <MdIcon :name="ICON_NAMES.refresh" :size="18" />
        {{ t('loading.processing') }}
      </div>
      <MdEmptyState
        v-else-if="searchResult && !visibleEntries.length"
        :icon-name="ICON_NAMES.search"
        :title="t('fileBrowser.noMatchesTitle')"
        description=""
        compact
      />
      <MdEmptyState
        v-else-if="!visibleEntries.length"
        :icon-name="ICON_NAMES.folder"
        :title="t('common.itemCount', { count: 0 })"
        description=""
        compact
      />
      <div v-else class="file-browser-entries">
        <button
          v-for="entry in visibleEntries"
          :key="entry.path"
          class="file-browser-entry"
          type="button"
          :disabled="busy"
          @click="activate(entry)"
        >
          <span class="file-browser-entry-icon">
            <MdIcon
              :name="entry.isSymlink ? ICON_NAMES.external : entry.isDirectory ? ICON_NAMES.folder : ICON_NAMES.file"
              :size="18"
            />
          </span>
          <span class="file-browser-entry-main">
            <strong>{{ entry.name }}</strong>
            <small dir="auto">{{
              searchResult ? entry.path : entry.isDirectory ? '' : ByteSizeService.bytes(entry.size)
            }}</small>
          </span>
          <small class="file-browser-entry-date">
            {{ entry.modifiedAtMs === null ? '—' : FormatService.dateTime(entry.modifiedAtMs, locale) }}
          </small>
        </button>
      </div>
    </MdResultWorkspace>

    <MdResultWorkspace v-else-if="busy" class="file-browser-workspace">
      <div class="file-browser-loading" role="status">
        <MdIcon :name="ICON_NAMES.refresh" :size="18" />
        {{ t('loading.processing') }}
      </div>
    </MdResultWorkspace>

    <MdResultWorkspace v-else class="file-browser-workspace">
      <MdEmptyState :icon-name="ICON_NAMES.folderSearch" :title="t('fileBrowser.emptyRootTitle')" description="">
        <Button data-testid="choose-folder-empty" variant="outline" @click="chooseFolder">
          <MdIcon :name="ICON_NAMES.folderOpen" :size="16" />
          {{ t('scanScope.chooseFolder') }}
        </Button>
      </MdEmptyState>
    </MdResultWorkspace>
  </MdPageShell>
</template>

<style scoped>
@reference "@assets/main.css";

.file-browser-workspace {
  display: flex;
  min-height: 0;
  flex-direction: column;
  gap: 12px;
}

.file-browser-toolbar,
.file-browser-navigation {
  display: flex;
  min-width: 0;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.file-browser-toolbar {
  width: 100%;
}

.file-browser-toolbar :deep(.result-search) {
  min-width: 180px;
  flex: 1;
}

.file-browser-path {
  overflow: hidden;
  border-radius: 8px;
  padding: 8px 11px;
  color: var(--muted-foreground);
  text-overflow: ellipsis;
  white-space: nowrap;
  @apply bg-muted/45;
}

.file-browser-summary,
.file-browser-notice,
.file-browser-error {
  margin: 0;
  font-size: var(--font-content-secondary);
}

.file-browser-summary,
.file-browser-notice {
  @apply text-muted-foreground;
}

.file-browser-error {
  @apply text-destructive-text;
}

.file-browser-entries {
  display: flex;
  min-height: 0;
  flex-direction: column;
  overflow: auto;
  scrollbar-width: thin;
}

.file-browser-entry {
  display: grid;
  min-height: 58px;
  grid-template-columns: 28px minmax(0, 1fr) minmax(110px, auto);
  align-items: center;
  gap: 10px;
  border: 0;
  border-bottom: 1px solid var(--border);
  padding: 7px 10px;
  text-align: start;
  @apply bg-transparent text-foreground;
}

.file-browser-entry:hover,
.file-browser-entry:focus-visible {
  @apply bg-muted/55;
}

.file-browser-entry-icon {
  display: grid;
  width: 28px;
  height: 28px;
  place-items: center;
  @apply text-primary-text;
}

.file-browser-entry-main {
  display: grid;
  min-width: 0;
  gap: 3px;
}

.file-browser-entry-main strong,
.file-browser-entry-main small,
.file-browser-entry-date {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-browser-entry-main strong {
  font-weight: 500;
}

.file-browser-entry-main small,
.file-browser-entry-date {
  @apply text-muted-foreground;
  font-size: var(--font-content-secondary);
}

.file-browser-entry-date {
  text-align: end;
}

.file-browser-loading {
  display: flex;
  min-height: 160px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  @apply text-muted-foreground;
}
</style>
