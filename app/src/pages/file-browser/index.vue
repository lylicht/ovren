<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import MdDestructiveActionDialog from '@/components/custom/md-destructive-action-dialog.vue';
import MdDialogContent from '@/components/custom/md-dialog-content.vue';
import MdDialogFooter from '@/components/custom/md-dialog-footer.vue';
import MdDialogHeader from '@/components/custom/md-dialog-header.vue';
import MdEmptyState from '@/components/custom/md-empty-state.vue';
import MdPageShell from '@/components/custom/md-page-shell.vue';
import MdResultSearch from '@/components/custom/md-result-search.vue';
import MdResultWorkspace from '@/components/custom/md-result-workspace.vue';
import MdIcon from '@/components/icons/md-icon.vue';
import { Button } from '@/components/ui/button';
import { Dialog, DialogDescription, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';

import type {
  FileBrowserEntry,
  FileBrowserSearchResult,
  FileMutationReport,
  FileTransferMode,
} from '@/lib/models/filesystem/file-browser';
import { ICON_NAMES } from '@/lib/models/presentation/ui';
import { FileManagerService } from '@/lib/services/filesystem/file-manager-service';
import { FolderSelectionService } from '@/lib/services/storage/folder-selection-service';
import { FormatService } from '@/lib/services/presentation/format-service';
import { ByteSizeService } from '@/lib/services/settings/byte-size-service';
import { browserName, browserSelectionRange, sortBrowserEntries, type BrowserSort, type BrowserView } from '@/lib/utils/filesystem/file-browser';
import { useStorageScopeStore } from '@/stores/storage-scope-store';
import { useAppStore } from '@/stores/app-store';

import MdFileBrowserEntries from './md-file-browser-entries.vue';

const emit = defineEmits<{
  analyze: [path: string];
  openTool: [page: 'large-files' | 'duplicate-files'];
}>();

const { locale, t } = useI18n({ useScope: 'global' });
const storageScopes = useStorageScopeStore();
const appStore = useAppStore();
const path = ref('');
const address = ref('');
const homePath = ref('');
const entries = ref<FileBrowserEntry[]>([]);
const selectedPaths = ref<string[]>([]);
const selectionAnchor = ref(-1);
const query = ref('');
const searchResult = ref<FileBrowserSearchResult | null>(null);
const busy = ref(false);
const errorMessage = ref('');
const statusMessage = ref('');
const history = ref<string[]>([]);
const historyIndex = ref(-1);
const clipboard = ref<{ paths: string[]; mode: FileTransferMode } | null>(null);
const showHidden = ref(true);
const sort = ref<BrowserSort>('name');
const descending = ref(false);
const view = ref<BrowserView>('details');
const createOpen = ref(false);
const createFolder = ref(true);
const createName = ref('');
const renameOpen = ref(false);
const renameName = ref('');
const trashOpen = ref(false);
const detailsOpen = ref(false);
const menu = ref<{ x: number; y: number } | null>(null);
const addressInput = ref<HTMLInputElement | null>(null);
let requestId = 0;
let componentActive = true;

const rawEntries = computed(() => searchResult.value?.entries ?? entries.value);
const visibleEntries = computed(() =>
  sortBrowserEntries(
    showHidden.value ? rawEntries.value : rawEntries.value.filter(entry => !entry.name.startsWith('.')),
    sort.value,
    descending.value,
    locale.value
  )
);
const selectedEntries = computed(() => {
  const selected = new Set(selectedPaths.value);
  return visibleEntries.value.filter(entry => selected.has(entry.path));
});
const selectedCount = computed(() => selectedPaths.value.length);
const selectedEntry = computed(() => (selectedEntries.value.length === 1 ? selectedEntries.value[0] : null));
const canBack = computed(() => historyIndex.value > 0);
const canForward = computed(() => historyIndex.value >= 0 && historyIndex.value < history.value.length - 1);
const canPaste = computed(() => Boolean(path.value && clipboard.value?.paths.length));
const recentFolders = computed(() => storageScopes.recentFolders.filter(folder => folder !== path.value).slice(0, 6));
const analysisPath = computed(() =>
  selectedEntry.value?.isDirectory && !selectedEntry.value.isSymlink ? selectedEntry.value.path : path.value
);

type HistoryMode = 'push' | 'replace' | 'index';

function clearMessages() {
  errorMessage.value = '';
  statusMessage.value = '';
}

async function loadDirectory(target: string, mode: HistoryMode = 'push', index = -1) {
  if (!target.trim()) return;
  const request = ++requestId;
  busy.value = true;
  menu.value = null;
  clearMessages();
  try {
    const listing = await FileManagerService.listBrowserDirectory(target);
    if (request !== requestId || !componentActive) return;
    path.value = listing.path;
    address.value = listing.path;
    entries.value = listing.entries;
    searchResult.value = null;
    query.value = '';
    selectedPaths.value = [];
    selectionAnchor.value = -1;
    if (mode === 'index') {
      historyIndex.value = index;
    } else if (mode === 'push') {
      if (history.value[historyIndex.value] !== listing.path) {
        history.value = [...history.value.slice(0, historyIndex.value + 1), listing.path].slice(-100);
        historyIndex.value = history.value.length - 1;
      }
    } else if (historyIndex.value >= 0) {
      history.value[historyIndex.value] = listing.path;
    }
    if (listing.skippedCount) {
      statusMessage.value = t('fileBrowser.skippedItems', { count: listing.skippedCount });
    }
  } catch {
    if (request === requestId) errorMessage.value = t('fileBrowser.directoryError');
  } finally {
    if (request === requestId) busy.value = false;
  }
}

async function chooseFolder() {
  if (busy.value) return;
  try {
    const [selected] = await FolderSelectionService.select(false, t('scanScope.chooseFolder'), path.value || undefined);
    if (selected) await loadDirectory(selected);
  } catch {
    errorMessage.value = t('common.operationFailed');
  }
}

async function search() {
  const needle = query.value.trim();
  if (!needle) {
    searchResult.value = null;
    selectedPaths.value = [];
    return;
  }
  if (!path.value || busy.value) return;
  const request = ++requestId;
  busy.value = true;
  clearMessages();
  try {
    const result = await FileManagerService.searchBrowserDirectory(path.value, needle);
    if (request !== requestId || !componentActive) return;
    searchResult.value = result;
    selectedPaths.value = [];
    selectionAnchor.value = -1;
  } catch {
    if (request === requestId) errorMessage.value = t('common.operationFailed');
  } finally {
    if (request === requestId) busy.value = false;
  }
}

async function refresh() {
  if (busy.value || !path.value) return;
  if (searchResult.value && query.value.trim()) {
    await search();
  } else {
    await loadDirectory(path.value, 'replace');
  }
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

async function goHistory(offset: number) {
  if (busy.value) return;
  const index = historyIndex.value + offset;
  const target = history.value[index];
  if (target) await loadDirectory(target, 'index', index);
}

async function goHome() {
  if (homePath.value && !busy.value) await loadDirectory(homePath.value);
}

async function navigateAddress() {
  if (busy.value) return;
  await loadDirectory(address.value);
}

async function activate(entry: FileBrowserEntry) {
  if (busy.value) return;
  if (entry.isDirectory && !entry.isSymlink) {
    await loadDirectory(entry.path);
  } else {
    try {
      await FileManagerService.openBrowserEntry(entry.path);
    } catch {
      errorMessage.value = t('common.operationFailed');
    }
  }
}

function select(entry: FileBrowserEntry, event: MouseEvent) {
  const index = visibleEntries.value.findIndex(item => item.path === entry.path);
  if (event.shiftKey && selectionAnchor.value >= 0) {
    const range = browserSelectionRange(visibleEntries.value, selectionAnchor.value, index);
    selectedPaths.value = event.ctrlKey || event.metaKey ? [...new Set([...selectedPaths.value, ...range])] : range;
    return;
  }
  if (event.ctrlKey || event.metaKey) {
    const next = new Set(selectedPaths.value);
    if (next.has(entry.path)) next.delete(entry.path);
    else next.add(entry.path);
    selectedPaths.value = [...next];
  } else {
    selectedPaths.value = [entry.path];
  }
  selectionAnchor.value = index;
}

function selectAll() {
  selectedPaths.value = visibleEntries.value.map(entry => entry.path);
}

function openCreate(folder: boolean) {
  if (!path.value || busy.value) return;
  createFolder.value = folder;
  createName.value = '';
  createOpen.value = true;
  menu.value = null;
}

function openRename() {
  if (selectedCount.value !== 1 || busy.value) return;
  renameName.value = selectedEntry.value?.name ?? '';
  renameOpen.value = true;
  menu.value = null;
}

function prepareTrash() {
  if (!selectedCount.value || busy.value) return;
  trashOpen.value = true;
  menu.value = null;
}

async function mutate(action: () => Promise<void>) {
  if (busy.value) return;
  busy.value = true;
  clearMessages();
  try {
    await action();
  } catch {
    errorMessage.value = t('common.operationFailed');
  } finally {
    busy.value = false;
    // Native mutations can change the directory even after a partial failure.
    await refresh();
  }
}

function reportResult(report: FileMutationReport) {
  const count = report.completed.length;
  if (report.failed.length) {
    const failure = report.failed[0];
    errorMessage.value = t('fileBrowser.partialFailure', {
      count: report.failed.length,
      reason: t('fileBrowser.errors.' + (failure?.reason ?? 'failed')),
    });
  }
  if (count) statusMessage.value = t('fileBrowser.completedCount', { count });
}

async function saveNew() {
  if (!createName.value.trim() || !path.value || busy.value) return;
  const name = createName.value.trim();
  createOpen.value = false;
  await mutate(async () => {
    await FileManagerService.createBrowserEntry(path.value, name, createFolder.value);
    statusMessage.value = t('fileBrowser.completedCount', { count: 1 });
  });
}

async function saveRename() {
  if (!renameName.value.trim() || !selectedEntry.value || busy.value) return;
  const target = selectedEntry.value.path;
  const name = renameName.value.trim();
  renameOpen.value = false;
  await mutate(async () => {
    await FileManagerService.renameBrowserEntry(target, name);
    statusMessage.value = t('fileBrowser.completedCount', { count: 1 });
  });
}

function copyOrCut(mode: FileTransferMode) {
  if (!selectedCount.value || busy.value) return;
  clipboard.value = { paths: [...selectedPaths.value], mode };
  statusMessage.value = t(mode === 'copy' ? 'fileBrowser.copyReady' : 'fileBrowser.cutReady', {
    count: selectedCount.value,
  });
  menu.value = null;
}

async function inBatches(
  paths: string[],
  action: (batch: string[]) => Promise<FileMutationReport>
): Promise<FileMutationReport> {
  const combined: FileMutationReport = { completed: [], failed: [] };
  for (let i = 0; i < paths.length; i += 128) {
    const result = await action(paths.slice(i, i + 128));
    combined.completed.push(...result.completed);
    combined.failed.push(...result.failed);
  }
  return combined;
}

async function paste() {
  const pending = clipboard.value;
  if (!pending?.paths.length || !path.value || busy.value) return;
  await mutate(async () => {
    const report = await inBatches(pending.paths, batch =>
      FileManagerService.transferBrowserEntries(batch, path.value, pending.mode)
    );
    reportResult(report);
    if (pending.mode === 'move') {
      clipboard.value = report.failed.length ? { paths: report.failed.map(failure => failure.path), mode: 'move' } : null;
    }
  });
}

async function trash() {
  if (!selectedCount.value || busy.value) return;
  const paths = [...selectedPaths.value];
  trashOpen.value = false;
  await mutate(async () => {
    const report = await inBatches(paths, batch => FileManagerService.trashBrowserEntries(batch));
    reportResult(report);
  });
}

function openContext(entry: FileBrowserEntry, event: MouseEvent) {
  if (busy.value) return;
  if (!selectedPaths.value.includes(entry.path)) {
    selectedPaths.value = [entry.path];
    selectionAnchor.value = visibleEntries.value.findIndex(item => item.path === entry.path);
  }
  menu.value = {
    x: Math.max(8, Math.min(event.clientX, window.innerWidth - 228)),
    y: Math.max(8, Math.min(event.clientY, window.innerHeight - 270)),
  };
}

function onKeydown(event: KeyboardEvent) {
  if (createOpen.value || renameOpen.value || trashOpen.value || detailsOpen.value) return;
  if (event.key === 'Escape') {
    menu.value = null;
    selectedPaths.value = [];
    return;
  }
  const target = event.target;
  const editing = target instanceof HTMLElement && Boolean(target.closest('input, textarea, [contenteditable="true"]'));
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'l') {
    event.preventDefault();
    addressInput.value?.focus();
    addressInput.value?.select();
    return;
  }
  if (editing || busy.value) return;
  const command = event.ctrlKey || event.metaKey;
  if (command && event.key.toLowerCase() === 'a') {
    event.preventDefault();
    selectAll();
  } else if (command && event.key.toLowerCase() === 'c') {
    event.preventDefault();
    copyOrCut('copy');
  } else if (command && event.key.toLowerCase() === 'x') {
    event.preventDefault();
    copyOrCut('move');
  } else if (command && event.key.toLowerCase() === 'v') {
    event.preventDefault();
    void paste();
  } else if (event.key === 'Delete' || (event.metaKey && event.key === 'Backspace')) {
    event.preventDefault();
    prepareTrash();
  } else if (event.key === 'F2') {
    event.preventDefault();
    openRename();
  } else if (event.key === 'Enter' && selectedEntry.value) {
    event.preventDefault();
    void activate(selectedEntry.value);
  } else if (event.altKey && event.key === 'ArrowLeft') {
    event.preventDefault();
    void goHistory(-1);
  } else if (event.altKey && event.key === 'ArrowRight') {
    event.preventDefault();
    void goHistory(1);
  } else if (event.altKey && event.key === 'ArrowUp') {
    event.preventDefault();
    void goUp();
  } else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault();
    const current = visibleEntries.value.findIndex(entry => entry.path === selectedPaths.value.at(-1));
    const next = Math.max(0, Math.min(visibleEntries.value.length - 1, current + (event.key === 'ArrowDown' ? 1 : -1)));
    const entry = visibleEntries.value[next];
    if (!entry) return;
    if (event.shiftKey && selectionAnchor.value >= 0) {
      selectedPaths.value = browserSelectionRange(visibleEntries.value, selectionAnchor.value, next);
    } else {
      selectedPaths.value = [entry.path];
      selectionAnchor.value = next;
    }
  }
}

watch(showHidden, () => {
  const visible = new Set(visibleEntries.value.map(entry => entry.path));
  selectedPaths.value = selectedPaths.value.filter(path => visible.has(path));
});

onMounted(() => {
  componentActive = true;
  window.addEventListener('keydown', onKeydown);
  void FileManagerService.homeDirectory()
    .then(home => {
      if (!componentActive) return;
      homePath.value = home;
      return loadDirectory(storageScopes.recentFolders[0] || home);
    })
    .catch(() => {
      errorMessage.value = t('fileBrowser.directoryError');
    });
});

onBeforeUnmount(() => {
  componentActive = false;
  ++requestId;
  window.removeEventListener('keydown', onKeydown);
});
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
      <Button variant="outline" :disabled="busy" @click="chooseFolder">
        <MdIcon :name="ICON_NAMES.folderOpen" :size="16" />
        {{ t('scanScope.chooseFolder') }}
      </Button>
    </template>

    <MdResultWorkspace class="file-browser-workspace">
      <div class="file-browser-nav" role="toolbar" :aria-label="t('fileBrowser.navigation')">
        <Button variant="ghost" size="icon" :disabled="busy || !canBack" :aria-label="t('fileBrowser.back')" @click="goHistory(-1)">
          <MdIcon :name="ICON_NAMES.chevronLeft" :size="17" />
        </Button>
        <Button variant="ghost" size="icon" :disabled="busy || !canForward" :aria-label="t('fileBrowser.forward')" @click="goHistory(1)">
          <MdIcon :name="ICON_NAMES.chevronRight" :size="17" />
        </Button>
        <Button variant="ghost" size="icon" :disabled="busy || !path" :aria-label="t('fileBrowser.goUp')" @click="goUp">
          <MdIcon :name="ICON_NAMES.chevronUp" :size="17" />
        </Button>
        <Button variant="ghost" size="icon" :disabled="busy || !homePath" :aria-label="t('fileBrowser.home')" @click="goHome">
          <MdIcon :name="ICON_NAMES.home" :size="17" />
        </Button>
        <form class="file-browser-address" @submit.prevent="navigateAddress">
          <Input ref="addressInput" v-model="address" :aria-label="t('fileBrowser.address')" :disabled="busy" />
        </form>
        <Button variant="ghost" size="icon" :disabled="busy || !path" :aria-label="t('monitoring.refresh')" @click="refresh">
          <MdIcon :name="ICON_NAMES.refresh" :size="17" />
        </Button>
      </div>

      <div class="file-browser-actions">
        <div class="file-browser-action-group">
          <Button variant="outline" size="sm" :disabled="busy || !path" @click="openCreate(true)">
            <MdIcon :name="ICON_NAMES.folderPlus" :size="15" />
            {{ t('fileBrowser.newFolder') }}
          </Button>
          <Button variant="outline" size="sm" :disabled="busy || !path" @click="openCreate(false)">
            <MdIcon :name="ICON_NAMES.file" :size="15" />
            {{ t('fileBrowser.newFile') }}
          </Button>
          <Button variant="ghost" size="sm" :disabled="busy || selectedCount !== 1" @click="openRename">
            {{ t('fileBrowser.rename') }}
          </Button>
          <Button variant="ghost" size="sm" :disabled="busy || !selectedCount" @click="copyOrCut('copy')">
            <MdIcon :name="ICON_NAMES.copy" :size="15" />
            {{ t('common.copy') }}
          </Button>
          <Button variant="ghost" size="sm" :disabled="busy || !selectedCount" @click="copyOrCut('move')">
            {{ t('fileBrowser.cut') }}
          </Button>
          <Button variant="ghost" size="sm" :disabled="busy || !canPaste" @click="paste">
            {{ t('fileBrowser.paste') }}
          </Button>
          <Button variant="ghost" size="sm" :disabled="busy || !selectedCount" @click="prepareTrash">
            <MdIcon :name="ICON_NAMES.trash" :size="15" />
            {{ t('fileBrowser.moveToTrash') }}
          </Button>
        </div>
        <div class="file-browser-view-group">
          <Button variant="ghost" size="icon" :aria-label="t('fileBrowser.detailsView')" :aria-pressed="view === 'details'" @click="view = 'details'">
            <MdIcon :name="ICON_NAMES.list" :size="17" />
          </Button>
          <Button variant="ghost" size="icon" :aria-label="t('fileBrowser.gridView')" :aria-pressed="view === 'grid'" @click="view = 'grid'">
            <MdIcon :name="ICON_NAMES.grid" :size="17" />
          </Button>
        </div>
      </div>

      <div class="file-browser-search-bar">
        <form class="file-browser-search-form" @submit.prevent="search">
          <MdResultSearch v-model="query" :placeholder="t('fileBrowser.search')" :disabled="busy || !path" />
          <Button size="sm" type="submit" :disabled="busy || !path">
            <MdIcon :name="ICON_NAMES.search" :size="15" />
            {{ t('fileBrowser.search') }}
          </Button>
          <Button v-if="searchResult" variant="ghost" size="sm" type="button" @click="searchResult = null; query = ''; selectedPaths = []">
            {{ t('common.close') }}
          </Button>
        </form>
        <div class="file-browser-display-options">
          <label class="file-browser-hidden-toggle">
            <input v-model="showHidden" type="checkbox" />
            {{ t('fileBrowser.hiddenFiles') }}
          </label>
          <select v-model="sort" :aria-label="t('fileBrowser.sortBy')" class="file-browser-sort">
            <option value="name">{{ t('fileBrowser.sortName') }}</option>
            <option value="size">{{ t('fileBrowser.sortSize') }}</option>
            <option value="modified">{{ t('fileBrowser.sortModified') }}</option>
          </select>
          <Button variant="ghost" size="icon" :aria-label="t('fileBrowser.reverseSort')" @click="descending = !descending">
            <MdIcon :name="ICON_NAMES.arrowUpDown" :size="15" />
          </Button>
        </div>
      </div>

      <p v-if="errorMessage" class="file-browser-error" role="alert">{{ errorMessage }}</p>
      <p v-else-if="statusMessage" class="file-browser-message" role="status">{{ statusMessage }}</p>
      <p v-if="searchResult?.truncated" class="file-browser-message" role="status">{{ t('fileBrowser.resultsLimited') }}</p>
      <p v-if="searchResult?.skippedCount" class="file-browser-message" role="status">
        {{ t('fileBrowser.skippedItems', { count: searchResult.skippedCount }) }}
      </p>

      <div class="file-browser-main">
        <aside class="file-browser-sidebar" :aria-label="t('fileBrowser.locations')">
          <div class="file-browser-side-section">
            <strong>{{ t('fileBrowser.locations') }}</strong>
            <button v-if="homePath" class="file-browser-side-link" type="button" :disabled="busy" @click="goHome">
              <MdIcon :name="ICON_NAMES.home" :size="16" />
              {{ t('fileBrowser.home') }}
            </button>
            <button v-for="volume in appStore.disks" :key="volume.mountPoint" class="file-browser-side-link" type="button" :disabled="busy" @click="loadDirectory(volume.mountPoint)">
              <MdIcon :name="ICON_NAMES.hardDrive" :size="16" />
              <span dir="auto">{{ volume.name }}</span>
            </button>
          </div>
          <div v-if="recentFolders.length" class="file-browser-side-section">
            <strong>{{ t('fileBrowser.recentFolders') }}</strong>
            <button v-for="recent in recentFolders" :key="recent" class="file-browser-side-link" type="button" :disabled="busy" @click="loadDirectory(recent)">
              <MdIcon :name="ICON_NAMES.folderClock" :size="16" />
              <span dir="auto">{{ browserName(recent) }}</span>
            </button>
          </div>
          <div class="file-browser-side-section">
            <strong>{{ t('fileBrowser.ovrenTools') }}</strong>
            <button class="file-browser-side-link" type="button" :disabled="busy || !analysisPath" @click="emit('analyze', analysisPath)">
              <MdIcon :name="ICON_NAMES.analysis" :size="16" />
              {{ t('navigation.analysis') }}
            </button>
            <button class="file-browser-side-link" type="button" @click="emit('openTool', 'large-files')">
              <MdIcon :name="ICON_NAMES.largeFiles" :size="16" />
              {{ t('navigation.large-files') }}
            </button>
            <button class="file-browser-side-link" type="button" @click="emit('openTool', 'duplicate-files')">
              <MdIcon :name="ICON_NAMES.duplicateFiles" :size="16" />
              {{ t('navigation.duplicate-files') }}
            </button>
          </div>
        </aside>

        <section class="file-browser-results" :aria-label="t('fileBrowser.items')">
          <div v-if="view === 'details' && visibleEntries.length" class="file-browser-column-headers">
            <span>{{ t('fileBrowser.sortName') }}</span>
            <span>{{ t('fileBrowser.sortSize') }}</span>
            <span>{{ t('fileBrowser.sortModified') }}</span>
          </div>
          <div v-if="busy && !entries.length" class="file-browser-center" role="status">
            <MdIcon :name="ICON_NAMES.refresh" :size="18" />
            {{ t('loading.processing') }}
          </div>
          <MdEmptyState
            v-else-if="!path"
            :icon-name="ICON_NAMES.folderSearch"
            :title="t('fileBrowser.emptyRootTitle')"
            description=""
            compact
          >
            <Button variant="outline" @click="chooseFolder">{{ t('scanScope.chooseFolder') }}</Button>
          </MdEmptyState>
          <MdEmptyState
            v-else-if="!visibleEntries.length"
            :icon-name="searchResult ? ICON_NAMES.search : ICON_NAMES.folder"
            :title="searchResult ? t('fileBrowser.noMatchesTitle') : t('fileBrowser.emptyFolder')"
            description=""
            compact
          />
          <MdFileBrowserEntries
            v-else
            :entries="visibleEntries"
            :selected-paths="selectedPaths"
            :view="view"
            :busy="busy"
            :search-active="Boolean(searchResult)"
            @select="select"
            @activate="activate"
            @context="openContext"
          />
        </section>

        <aside v-if="selectedEntry" class="file-browser-details" :aria-label="t('fileBrowser.properties')">
          <MdIcon :name="selectedEntry.isDirectory ? ICON_NAMES.folder : ICON_NAMES.file" :size="40" />
          <strong dir="auto">{{ selectedEntry.name }}</strong>
          <dl>
            <dt>{{ t('fileBrowser.location') }}</dt>
            <dd dir="auto">{{ selectedEntry.path }}</dd>
            <dt>{{ t('fileBrowser.kind') }}</dt>
            <dd>{{ selectedEntry.isSymlink ? t('fileBrowser.linkKind') : selectedEntry.isDirectory ? t('fileBrowser.folderKind') : t('fileBrowser.fileKind') }}</dd>
            <dt>{{ t('fileBrowser.sortSize') }}</dt>
            <dd>{{ ByteSizeService.bytes(selectedEntry.size) }}</dd>
            <dt>{{ t('fileBrowser.sortModified') }}</dt>
            <dd>{{ selectedEntry.modifiedAtMs === null ? '—' : FormatService.dateTime(selectedEntry.modifiedAtMs, locale) }}</dd>
          </dl>
          <Button variant="outline" size="sm" :disabled="busy" @click="activate(selectedEntry)">{{ t('common.open') }}</Button>
          <Button variant="ghost" size="sm" :disabled="busy" @click="FileManagerService.reveal(selectedEntry.path).catch(() => errorMessage = t('common.operationFailed'))">{{ t('common.showInFileManager') }}</Button>
        </aside>
      </div>

      <footer class="file-browser-status">
        <span>{{ t('common.itemCount', { count: visibleEntries.length }, visibleEntries.length) }}</span>
        <span v-if="selectedCount">{{ t('fileBrowser.selectedCount', { count: selectedCount }) }}</span>
        <span v-if="clipboard">{{ t('fileBrowser.clipboardCount', { count: clipboard.paths.length }) }}</span>
        <span v-if="busy" aria-live="polite">{{ t('loading.processing') }}</span>
      </footer>
    </MdResultWorkspace>

    <div v-if="menu" class="file-browser-context-overlay" @click="menu = null" @contextmenu.prevent="menu = null">
      <div
        class="file-browser-context"
        role="menu"
        :style="{ left: menu.x + 'px', top: menu.y + 'px' }"
        @click.stop
      >
        <button role="menuitem" type="button" :disabled="!selectedEntry" @click="selectedEntry && activate(selectedEntry); menu = null">{{ t('common.open') }}</button>
        <button role="menuitem" type="button" @click="copyOrCut('copy')">{{ t('common.copy') }}</button>
        <button role="menuitem" type="button" @click="copyOrCut('move')">{{ t('fileBrowser.cut') }}</button>
        <button role="menuitem" type="button" :disabled="selectedCount !== 1" @click="openRename">{{ t('fileBrowser.rename') }}</button>
        <button role="menuitem" type="button" @click="prepareTrash">{{ t('fileBrowser.moveToTrash') }}</button>
        <button role="menuitem" type="button" :disabled="selectedCount !== 1" @click="detailsOpen = true; menu = null">{{ t('fileBrowser.properties') }}</button>
      </div>
    </div>

    <Dialog :open="createOpen" @update:open="createOpen = $event">
      <MdDialogContent>
        <MdDialogHeader>
          <DialogTitle>{{ t(createFolder ? 'fileBrowser.newFolder' : 'fileBrowser.newFile') }}</DialogTitle>
          <DialogDescription>{{ t('fileBrowser.createDescription') }}</DialogDescription>
        </MdDialogHeader>
        <form class="file-browser-dialog-body" @submit.prevent="saveNew">
          <Input v-model="createName" autofocus :aria-label="t('fileBrowser.sortName')" maxlength="240" />
          <MdDialogFooter>
            <Button type="button" variant="outline" @click="createOpen = false">{{ t('common.cancel') }}</Button>
            <Button type="submit" :disabled="!createName.trim()">{{ t('fileBrowser.create') }}</Button>
          </MdDialogFooter>
        </form>
      </MdDialogContent>
    </Dialog>

    <Dialog :open="renameOpen" @update:open="renameOpen = $event">
      <MdDialogContent>
        <MdDialogHeader>
          <DialogTitle>{{ t('fileBrowser.rename') }}</DialogTitle>
          <DialogDescription>{{ t('fileBrowser.renameDescription') }}</DialogDescription>
        </MdDialogHeader>
        <form class="file-browser-dialog-body" @submit.prevent="saveRename">
          <Input v-model="renameName" autofocus :aria-label="t('fileBrowser.sortName')" maxlength="240" />
          <MdDialogFooter>
            <Button type="button" variant="outline" @click="renameOpen = false">{{ t('common.cancel') }}</Button>
            <Button type="submit" :disabled="!renameName.trim()">{{ t('common.save') }}</Button>
          </MdDialogFooter>
        </form>
      </MdDialogContent>
    </Dialog>

    <MdDestructiveActionDialog
      :open="trashOpen"
      :busy="busy"
      :title="t('fileBrowser.moveToTrash')"
      :description="t('fileBrowser.trashDescription')"
      :summary-label="t('fileBrowser.selectedCount', { count: selectedCount })"
      :summary-value="selectedCount === 1 ? selectedEntry?.name : ''"
      :cancel-label="t('common.cancel')"
      :confirm-label="t('fileBrowser.moveToTrash')"
      @update:open="trashOpen = $event"
      @confirm="trash"
    />

    <Dialog :open="detailsOpen" @update:open="detailsOpen = $event">
      <MdDialogContent>
        <MdDialogHeader>
          <DialogTitle>{{ t('fileBrowser.properties') }}</DialogTitle>
          <DialogDescription>{{ selectedEntry?.name ?? '' }}</DialogDescription>
        </MdDialogHeader>
        <div class="file-browser-dialog-body">
          <p dir="auto">{{ selectedEntry?.path }}</p>
        </div>
        <MdDialogFooter>
          <Button variant="outline" @click="detailsOpen = false">{{ t('common.close') }}</Button>
        </MdDialogFooter>
      </MdDialogContent>
    </Dialog>
  </MdPageShell>
</template>

<style scoped>
@reference "@assets/main.css";
.file-browser-workspace { display: flex; min-height: 0; flex-direction: column; gap: 8px; height: 100%; }
.file-browser-nav,.file-browser-actions,.file-browser-search-bar { display: flex; min-width: 0; align-items: center; gap: 6px; }
.file-browser-nav { padding: 4px 2px; }
.file-browser-nav :deep(button) { flex: none; }
.file-browser-address { flex: 1; min-width: 100px; }
.file-browser-address :deep(input) { font-family: inherit; }
.file-browser-actions { justify-content: space-between; gap: 10px; flex-wrap: wrap; }
.file-browser-action-group,.file-browser-view-group { display: flex; align-items: center; gap: 3px; flex-wrap: wrap; }
.file-browser-view-group :deep(button[aria-pressed='true']) { @apply bg-primary/15 text-primary-text; }
.file-browser-search-bar { justify-content: space-between; flex-wrap: wrap; }
.file-browser-search-form { display: flex; flex: 1; min-width: 200px; align-items: center; gap: 5px; }
.file-browser-search-form :deep(.result-search) { min-width: 120px; flex: 1; }
.file-browser-display-options { display: flex; align-items: center; gap: 8px; }
.file-browser-hidden-toggle { display: flex; align-items: center; gap: 6px; white-space: nowrap; font-size: var(--font-content-secondary); }
.file-browser-sort { max-width: 130px; border: 1px solid var(--border); border-radius: 7px; padding: 5px; font-size: var(--font-content-secondary); @apply bg-background text-foreground; }
.file-browser-error,.file-browser-message { margin: 0; padding: 5px 9px; font-size: var(--font-content-secondary); }
.file-browser-error { @apply text-destructive-text; }
.file-browser-message { @apply text-muted-foreground; }
.file-browser-main { display: grid; grid-template-columns: 185px minmax(0,1fr); flex: 1; min-height: 0; overflow: hidden; border: 1px solid var(--border); border-radius: 9px; }
.file-browser-main:has(.file-browser-details) { grid-template-columns: 165px minmax(0,1fr) 220px; }
.file-browser-sidebar { display: flex; flex-direction: column; gap: 15px; padding: 12px 8px; overflow-y: auto; border-right: 1px solid var(--border); @apply bg-muted/20; }
.file-browser-side-section { display: flex; min-width: 0; flex-direction: column; gap: 3px; }
.file-browser-side-section > strong { padding: 6px 8px; font-size: var(--font-content-secondary); @apply text-muted-foreground; }
.file-browser-side-link { display: flex; align-items: center; gap: 8px; width: 100%; min-width: 0; border: 0; border-radius: 6px; padding: 7px 8px; text-align: start; font-size: var(--font-content-secondary); @apply bg-transparent text-foreground; }
.file-browser-side-link svg { flex: none; @apply text-primary-text; }
.file-browser-side-link span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.file-browser-side-link:hover,.file-browser-side-link:focus-visible { @apply bg-muted; }
.file-browser-results { display: flex; flex-direction: column; overflow: hidden; min-width: 0; min-height: 0; }
.file-browser-column-headers { display: grid; grid-template-columns: minmax(0,1fr) 105px 160px; gap: 10px; padding: 8px 13px 8px 52px; border-bottom: 1px solid var(--border); font-size: var(--font-content-secondary); @apply bg-muted/20 text-muted-foreground; }
.file-browser-column-headers span:not(:first-child) { text-align: end; }
.file-browser-center { display: flex; flex: 1; align-items: center; justify-content: center; gap: 10px; @apply text-muted-foreground; }
.file-browser-details { display: flex; flex-direction: column; gap: 9px; align-items: center; overflow: auto; padding: 18px 10px; border-left: 1px solid var(--border); }
.file-browser-details > svg { @apply text-primary-text; }
.file-browser-details > strong { max-width: 100%; overflow-wrap: anywhere; }
.file-browser-details dl { width: 100%; }
.file-browser-details dt { padding-top: 9px; font-size: var(--font-content-secondary); @apply text-muted-foreground; }
.file-browser-details dd { margin: 2px 0; overflow-wrap: anywhere; font-size: var(--font-content-secondary); }
.file-browser-details :deep(button) { width: 100%; }
.file-browser-status { display: flex; flex-wrap: wrap; gap: 14px; align-items: center; padding: 4px 9px; font-size: var(--font-content-secondary); @apply text-muted-foreground; }
.file-browser-context-overlay { position: fixed; z-index: 80; inset: 0; }
.file-browser-context { position: fixed; display: flex; flex-direction: column; width: 220px; gap: 2px; border: 1px solid var(--border); border-radius: 8px; padding: 5px; box-shadow: 0 5px 18px var(--shadow-subtle); @apply bg-popover; }
.file-browser-context button { border: 0; border-radius: 6px; padding: 8px 10px; text-align: start; font-size: var(--font-content-body); @apply bg-transparent text-popover-foreground; }
.file-browser-context button:hover:not(:disabled) { @apply bg-muted; }
.file-browser-context button:disabled { opacity: .45; }
.file-browser-dialog-body { display: flex; flex-direction: column; gap: 14px; padding: 12px var(--layout-dialog-body-inline-padding); }
.file-browser-dialog-body :deep(.md-dialog-footer) { margin: 0 calc(-1 * var(--layout-dialog-body-inline-padding)) -12px; }
@container file-browser (max-width: 1050px) {
 .file-browser-main { grid-template-columns: 155px minmax(0,1fr); }
 .file-browser-main:has(.file-browser-details) { grid-template-columns: 135px minmax(0,1fr); }
 .file-browser-details { display: none; }
 .file-browser-column-headers { grid-template-columns: minmax(0,1fr) 105px; }
 .file-browser-column-headers span:last-child { display: none; }
}
@container file-browser (max-width: 670px) {
 .file-browser-main,.file-browser-main:has(.file-browser-details) { grid-template-columns: minmax(0,1fr); }
 .file-browser-sidebar { display: none; }
 .file-browser-column-headers { grid-template-columns: minmax(0,1fr) 105px; }
}
</style>
