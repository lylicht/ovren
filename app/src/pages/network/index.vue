<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { Button } from '@/components/ui/button';
import MdPageShell from '@/components/custom/md-page-shell.vue';
import { METRIC_STATUS_KEYS } from '@/lib/models/system-resources/system-resources';
import { LoggerService } from '@/lib/services/diagnostics/logger-service';
import { ByteSizeService } from '@/lib/services/settings/byte-size-service';
import { ResidentService } from '@/lib/services/resident/resident-service';
import { WyreNetworkService, type NetworkMeasurement } from '@/lib/services/network/wyre-network-service';
import { OvrenNetService, type OvrenNetSnapshot } from '@/lib/services/network/ovren-net-service';
import { OvrenNetPreferencesService } from '@/lib/services/network/ovren-net-preferences-service';
import type { OvrenNetPreferences } from '@/lib/models/network/ovren-net-preferences';
import { useTrayPanelStore } from '@/stores/tray-panel-store';

const { t, locale } = useI18n({ useScope: 'global' });
const trayPanelStore = useTrayPanelStore();
const busy = ref(false);
const result = ref<NetworkMeasurement | null>(null);
const error = ref(false);
const ovrenNet = ref<OvrenNetSnapshot | null>(null);
const preferences = ref<OvrenNetPreferences>(OvrenNetPreferencesService.defaults());
const preferencesReady = ref(false);
const preferencesError = ref(false);
const ovrenNetBusy = ref(false);
const ovrenNetLoading = ref(false);
const ovrenNetError = ref(false);
const ovrenNetAdapters = computed(() => ovrenNet.value?.adapters ?? []);
const includedAdapterCount = computed(
  () => ovrenNetAdapters.value.filter(adapter => isAdapterIncluded(adapter.name)).length
);
const selectedAdapterCount = computed(() => preferences.value.includedAdapters.length || ovrenNetAdapters.value.length);
const ovrenNetState = computed(() => {
  if (!ovrenNet.value?.running) return t('ovrenNet.statusStopped');
  if (ovrenNet.value.pausedReason === 'manual' || ovrenNet.value.pausedReason === 'automatic') {
    return t('ovrenNet.statusPaused');
  }
  return ovrenNet.value.enabled ? t('ovrenNet.statusActive') : t('ovrenNet.statusStarting');
});
const networkReading = computed(() => trayPanelStore.reading.network);
const network = computed(() =>
  ['ready', 'stale'].includes(networkReading.value.status) ? networkReading.value.value : null
);
const networkStatus = computed(() => t(METRIC_STATUS_KEYS[networkReading.value.status]));
const connectionStatus = computed(() =>
  network.value
    ? t(network.value.interface.connected ? 'networkDetails.connected' : 'systemStatus.disconnected')
    : networkStatus.value
);
const ipv4Addresses = computed(
  () => network.value?.interface.details?.addresses.filter(address => !address.includes(':')) ?? []
);

let disposed = false;
let unlistenReading: (() => void) | null = null;
let ovrenNetTimer: number | null = null;
onMounted(() => {
  void trayPanelStore.load();
  void loadOvrenNet();
  ovrenNetTimer = window.setInterval(() => void refreshOvrenNet(), 2000);
  void ResidentService.onReading(reading => trayPanelStore.accept(reading))
    .then(unlisten => {
      if (disposed) unlisten();
      else unlistenReading = unlisten;
    })
    .catch(cause => LoggerService.warn('network', 'monitor_subscription_failed', { error: cause }));
});
onUnmounted(() => {
  disposed = true;
  if (ovrenNetTimer !== null) window.clearInterval(ovrenNetTimer);
  unlistenReading?.();
});

async function loadOvrenNet() {
  try {
    preferences.value = await OvrenNetPreferencesService.load();
  } catch (cause) {
    preferencesError.value = true;
    ovrenNetError.value = true;
    LoggerService.warn('network', 'ovren_net_preferences_load_failed', { error: cause });
  } finally {
    preferencesReady.value = true;
  }
  await refreshOvrenNet();
}

async function refreshOvrenNet() {
  if (!preferencesReady.value || ovrenNetLoading.value) return;
  ovrenNetLoading.value = true;
  try {
    ovrenNet.value = await OvrenNetService.snapshot(preferences.value);
    ovrenNetError.value = preferencesError.value || ovrenNet.value.configError !== '';
  } catch (cause) {
    if (!ovrenNetError.value) LoggerService.warn('network', 'ovren_net_status_failed', { error: cause });
    ovrenNetError.value = true;
  } finally {
    ovrenNetLoading.value = false;
  }
}

async function savePreferences(next: OvrenNetPreferences) {
  if (ovrenNetBusy.value) return;
  ovrenNetBusy.value = true;
  ovrenNetError.value = false;
  try {
    const validated = OvrenNetPreferencesService.parse(next);
    await OvrenNetPreferencesService.save(validated);
    preferencesError.value = false;
    preferences.value = validated;
    if (ovrenNet.value?.running) await OvrenNetService.configure(validated);
    if (ovrenNet.value?.socksRunning) {
      await OvrenNetService.socksStop();
      await OvrenNetService.socksStart(validated);
    }
    await refreshOvrenNet();
  } catch (cause) {
    ovrenNetError.value = true;
    LoggerService.warn('network', 'ovren_net_preferences_save_failed', { error: cause });
  } finally {
    ovrenNetBusy.value = false;
  }
}

function setIncludedAdapter(name: string, included: boolean) {
  const available = ovrenNetAdapters.value.map(adapter => adapter.name);
  const selected = new Set(preferences.value.includedAdapters.length ? preferences.value.includedAdapters : available);
  if (!included && selected.has(name) && selected.size === 1) return;
  if (included) selected.add(name);
  else selected.delete(name);
  const unknown = preferences.value.includedAdapters.filter(adapter => !available.includes(adapter));
  const includedAdapters =
    available.every(adapter => selected.has(adapter)) && unknown.length === 0
      ? []
      : [...new Set([...unknown, ...selected])];
  void savePreferences({ ...preferences.value, includedAdapters });
}

function isAdapterIncluded(name: string) {
  return preferences.value.includedAdapters.length === 0 || preferences.value.includedAdapters.includes(name);
}

function setAdapterNumber(key: 'weights' | 'limitsMbps', name: string, event: Event) {
  const value = Number((event.target as HTMLInputElement).value);
  if (!Number.isFinite(value)) {
    ovrenNetError.value = true;
    return;
  }
  void savePreferences({
    ...preferences.value,
    [key]: { ...preferences.value[key], [name]: value },
  });
}

async function runOvrenNetAction(
  action: 'start' | 'pause' | 'resume' | 'stop' | 'recover' | 'socksStart' | 'socksStop'
) {
  if (ovrenNetBusy.value) return;
  ovrenNetBusy.value = true;
  ovrenNetError.value = false;
  try {
    if (action === 'start') await OvrenNetService.start(preferences.value);
    else if (action === 'pause') await OvrenNetService.pause();
    else if (action === 'resume') await OvrenNetService.resume();
    else if (action === 'stop') await OvrenNetService.stop();
    else if (action === 'recover') await OvrenNetService.recover();
    else if (action === 'socksStart') await OvrenNetService.socksStart(preferences.value);
    else await OvrenNetService.socksStop();
    await refreshOvrenNet();
  } catch (cause) {
    ovrenNetError.value = true;
    LoggerService.warn('network', `ovren_net_${action}_failed`, { error: cause });
  } finally {
    ovrenNetBusy.value = false;
  }
}

function formatRate(bytesPerSecond: number): string {
  return Number.isFinite(bytesPerSecond) && bytesPerSecond >= 0 ? `${ByteSizeService.bytes(bytesPerSecond)}/s` : '—';
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat(locale.value, { maximumFractionDigits: 1 }).format(value);
}

const measurements = computed(() => {
  if (!result.value) return [];
  return [
    { key: 'download', label: t('wyre.download'), value: `${formatNumber(result.value.downloadMbps)} Mbps` },
    { key: 'upload', label: t('wyre.upload'), value: `${formatNumber(result.value.uploadMbps)} Mbps` },
    { key: 'latency', label: t('wyre.latency'), value: `${result.value.latencyMs} ms` },
    { key: 'jitter', label: t('wyre.jitter'), value: `${result.value.jitterMs} ms` },
    { key: 'failedProbes', label: t('wyre.failedProbes'), value: `${result.value.failedProbesPercent}%` },
  ];
});

async function measure() {
  if (busy.value) return;
  busy.value = true;
  error.value = false;
  result.value = null;
  try {
    result.value = await WyreNetworkService.measure();
  } catch (cause) {
    error.value = true;
    LoggerService.warn('network', 'speed_check_failed', { error: cause });
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <MdPageShell
    :title="t('wyre.networkTitle')"
    :subtitle="t('wyre.networkDescription')"
    content-mode="document"
    hide-scrollbar
  >
    <template #actions>
      <Button :disabled="busy" :aria-busy="busy" @click="measure">
        {{ busy ? t('wyre.networkMeasuring') : t('wyre.networkRun') }}
      </Button>
    </template>

    <div class="grid content-start gap-6 xl:grid-cols-[minmax(0,1.35fr)_minmax(280px,.65fr)]">
      <section class="min-w-0 rounded-xl border bg-card p-6" :aria-label="t('wyre.networkTestTitle')">
        <header class="mb-4 flex flex-wrap items-start justify-between gap-4">
          <div class="min-w-0">
            <h2 class="m-0 text-base font-medium text-foreground">{{ t('wyre.networkTestTitle') }}</h2>
            <p class="mb-0 mt-1 text-sm leading-relaxed text-muted-foreground">
              {{
                result
                  ? t('wyre.networkResultNote', { amount: formatNumber(result.dataMb) })
                  : t('wyre.networkDescription')
              }}
            </p>
          </div>
          <span
            v-if="busy"
            class="rounded-full border border-primary/25 bg-primary/10 px-3 py-1 text-xs text-primary-text"
            role="status"
            aria-live="polite"
          >
            {{ t('wyre.networkMeasuring') }}
          </span>
        </header>

        <dl v-if="result" class="m-0 grid grid-cols-2 gap-4 lg:grid-cols-6">
          <div
            v-for="item in measurements"
            :key="item.key"
            class="min-w-0 rounded-lg border bg-background p-4"
            :class="item.key === 'download' || item.key === 'upload' ? 'lg:col-span-3' : 'lg:col-span-2'"
          >
            <dt class="text-xs text-muted-foreground">{{ item.label }}</dt>
            <dd class="mb-0 mt-2 truncate text-2xl font-medium tabular-nums text-foreground">{{ item.value }}</dd>
          </div>
        </dl>

        <div v-if="result" class="mt-4 flex flex-wrap gap-x-5 gap-y-2 text-sm text-muted-foreground" aria-live="polite">
          <span
            >{{ t('wyre.networkTestServer') }}: <code class="text-foreground">{{ result.serverHost }}</code></span
          >
          <span
            >{{ t('wyre.networkTestEdge') }}:
            <code class="text-foreground">{{ result.serverLocation || t('wyre.networkTestUnknown') }}</code></span
          >
        </div>

        <div
          v-else
          class="grid min-h-64 content-center justify-items-center rounded-lg border border-dashed bg-background px-6 py-10 text-center"
          :class="{ 'border-destructive/40': error }"
          role="status"
          aria-live="polite"
        >
          <span
            class="mb-4 grid size-11 place-items-center rounded-full border bg-primary/10 text-primary-text"
            aria-hidden="true"
          >
            <span class="size-2.5 rounded-full bg-primary" :class="{ 'animate-pulse': busy }"></span>
          </span>
          <h3 class="m-0 text-base font-medium text-foreground">
            {{ error ? t('wyre.networkErrorTitle') : busy ? t('wyre.networkMeasuring') : t('wyre.networkReadyTitle') }}
          </h3>
          <p class="mb-0 mt-2 max-w-md text-sm leading-relaxed text-muted-foreground">
            {{
              error
                ? t('wyre.networkErrorDescription')
                : busy
                  ? t('wyre.networkDescription')
                  : t('wyre.networkReadyDescription')
            }}
          </p>
        </div>
      </section>

      <div class="grid content-start gap-6">
        <section class="min-w-0 rounded-xl border bg-card p-6" :aria-label="t('wyre.networkCurrentConnection')">
          <div class="flex items-start justify-between gap-4">
            <div class="min-w-0">
              <h2 class="m-0 text-base font-medium text-foreground">{{ t('wyre.networkCurrentConnection') }}</h2>
              <p class="mb-0 mt-1 truncate text-sm text-muted-foreground">
                {{ network?.interface.name ?? networkStatus }}
              </p>
            </div>
            <span
              class="shrink-0 rounded-full border px-3 py-1 text-xs"
              :class="
                network?.interface.connected
                  ? 'border-primary/25 bg-primary/10 text-primary-text'
                  : 'bg-muted text-muted-foreground'
              "
            >
              {{ connectionStatus }}
            </span>
          </div>
          <dl class="mb-0 mt-4 grid grid-cols-2 gap-4">
            <div class="min-w-0 rounded-lg border bg-background p-3">
              <dt class="text-xs text-muted-foreground">{{ t('systemStatus.download') }}</dt>
              <dd class="mb-0 mt-2 truncate text-lg font-medium tabular-nums text-foreground">
                {{ network ? formatRate(network.receivedBytesPerSecond) : '—' }}
              </dd>
            </div>
            <div class="min-w-0 rounded-lg border bg-background p-3">
              <dt class="text-xs text-muted-foreground">{{ t('systemStatus.upload') }}</dt>
              <dd class="mb-0 mt-2 truncate text-lg font-medium tabular-nums text-foreground">
                {{ network ? formatRate(network.transmittedBytesPerSecond) : '—' }}
              </dd>
            </div>
          </dl>
          <div v-if="ipv4Addresses.length" class="mt-4 border-t pt-4">
            <p class="mb-1 text-xs text-muted-foreground">IPv4</p>
            <p class="mb-0 break-all font-mono text-sm text-foreground">{{ ipv4Addresses.join(', ') }}</p>
          </div>
          <p v-else class="mb-0 mt-4 text-sm text-muted-foreground" role="status">{{ networkStatus }}</p>
        </section>

        <aside class="rounded-xl border bg-card p-6">
          <h2 class="m-0 text-base font-medium text-foreground">{{ t('wyre.networkPrivacyTitle') }}</h2>
          <p class="mb-0 mt-2 text-sm leading-relaxed text-muted-foreground">{{ t('wyre.networkPrivacy') }}</p>
        </aside>
      </div>
    </div>

    <section class="min-w-0 rounded-xl border bg-card p-6" :aria-label="t('ovrenNet.title')">
      <header class="flex flex-wrap items-start justify-between gap-4">
        <div class="min-w-0">
          <h2 class="m-0 text-base font-medium text-foreground">{{ t('ovrenNet.title') }}</h2>
          <p class="mb-0 mt-1 max-w-3xl text-sm leading-relaxed text-muted-foreground">
            {{ t('ovrenNet.description') }}
          </p>
        </div>
        <div class="flex shrink-0 flex-wrap gap-2">
          <Button
            v-if="!ovrenNet?.running"
            :disabled="ovrenNetBusy || includedAdapterCount === 0 || !preferencesReady"
            :aria-busy="ovrenNetBusy"
            @click="runOvrenNetAction('start')"
          >
            {{ ovrenNetBusy ? t('ovrenNet.statusStarting') : t('ovrenNet.start') }}
          </Button>
          <template v-else>
            <Button
              :disabled="ovrenNetBusy"
              :aria-busy="ovrenNetBusy"
              variant="outline"
              @click="runOvrenNetAction(ovrenNet.enabled ? 'pause' : 'resume')"
            >
              {{ ovrenNet.enabled ? t('ovrenNet.pause') : t('ovrenNet.resume') }}
            </Button>
            <Button :disabled="ovrenNetBusy" variant="outline" @click="runOvrenNetAction('stop')">
              {{ t('ovrenNet.stop') }}
            </Button>
          </template>
        </div>
      </header>

      <div class="mt-4 flex flex-wrap items-center gap-x-4 gap-y-2 text-sm" role="status" aria-live="polite">
        <span class="font-medium text-foreground">{{ ovrenNetState }}</span>
        <span v-if="includedAdapterCount === 0" class="text-muted-foreground">{{ t('ovrenNet.noAdapters') }}</span>
        <span v-if="ovrenNet && !ovrenNet.running && includedAdapterCount === 1" class="text-muted-foreground">
          {{ t('ovrenNet.oneConnection') }}
        </span>
        <span v-if="ovrenNetError" class="text-destructive-text" role="alert">{{ t('ovrenNet.error') }}</span>
      </div>
      <p class="mb-0 mt-3 max-w-4xl text-sm leading-relaxed text-muted-foreground">{{ t('ovrenNet.trafficNotice') }}</p>

      <ul v-if="ovrenNetAdapters.length" class="mb-0 mt-6 grid list-none gap-4 p-0 md:grid-cols-2">
        <li v-for="adapter in ovrenNetAdapters" :key="adapter.name" class="min-w-0 rounded-lg border bg-background p-4">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <h3 class="m-0 min-w-0 truncate text-sm font-medium text-foreground">{{ adapter.name }}</h3>
            <span class="text-xs text-muted-foreground">{{
              adapter.connected ? t('networkDetails.connected') : t('systemStatus.disconnected')
            }}</span>
          </div>
          <dl class="mb-0 mt-3 grid grid-cols-2 gap-3 sm:grid-cols-4">
            <div class="min-w-0">
              <dt class="text-xs text-muted-foreground">{{ t('ovrenNet.forwardedDownload') }}</dt>
              <dd class="mb-0 mt-1 truncate text-sm font-medium tabular-nums text-foreground">
                {{ formatRate(adapter.downloadBytesPerSecond) }}
              </dd>
            </div>
            <div class="min-w-0">
              <dt class="text-xs text-muted-foreground">{{ t('ovrenNet.forwardedUpload') }}</dt>
              <dd class="mb-0 mt-1 truncate text-sm font-medium tabular-nums text-foreground">
                {{ formatRate(adapter.uploadBytesPerSecond) }}
              </dd>
            </div>
            <div class="min-w-0">
              <dt class="text-xs text-muted-foreground">{{ t('ovrenNet.allLinkDownload') }}</dt>
              <dd class="mb-0 mt-1 truncate text-sm font-medium tabular-nums text-foreground">
                {{ isAdapterIncluded(adapter.name) ? formatRate(adapter.linkDownloadBytesPerSecond) : '—' }}
              </dd>
            </div>
            <div class="min-w-0">
              <dt class="text-xs text-muted-foreground">{{ t('ovrenNet.allLinkUpload') }}</dt>
              <dd class="mb-0 mt-1 truncate text-sm font-medium tabular-nums text-foreground">
                {{ isAdapterIncluded(adapter.name) ? formatRate(adapter.linkUploadBytesPerSecond) : '—' }}
              </dd>
            </div>
          </dl>
          <p class="mb-0 mt-3 text-xs text-muted-foreground">
            {{ t('ovrenNet.activeFlows') }}: {{ formatNumber(adapter.activeFlows) }}/{{
              formatNumber(adapter.totalFlows)
            }}
          </p>
        </li>
      </ul>

      <section class="mt-6 border-t pt-6" :aria-label="t('ovrenNet.advancedTitle')">
        <h3 class="m-0 text-base font-medium text-foreground">{{ t('ovrenNet.advancedTitle') }}</h3>
        <p class="mb-0 mt-1 text-sm text-muted-foreground">{{ t('ovrenNet.advancedDescription') }}</p>
        <div class="mt-4 grid gap-4 md:grid-cols-2">
          <label class="flex items-center gap-3 rounded-lg border bg-background p-3 text-sm">
            <input
              type="checkbox"
              class="size-4 accent-primary"
              :checked="preferences.autoWeight"
              :disabled="ovrenNetBusy"
              @change="savePreferences({ ...preferences, autoWeight: ($event.target as HTMLInputElement).checked })"
            />
            <span>{{ t('ovrenNet.adapterAutoWeight') }}</span>
          </label>
          <label class="flex items-center gap-3 rounded-lg border bg-background p-3 text-sm">
            <input
              type="checkbox"
              class="size-4 accent-primary"
              :checked="preferences.autoPause"
              :disabled="ovrenNetBusy"
              @change="savePreferences({ ...preferences, autoPause: ($event.target as HTMLInputElement).checked })"
            />
            <span>{{ t('ovrenNet.autoPause') }}</span>
          </label>
          <label class="flex items-center justify-between gap-3 rounded-lg border bg-background p-3 text-sm">
            <span>{{ t('ovrenNet.ipv6Mode') }}</span>
            <select
              class="h-9 max-w-40 rounded-md border bg-background px-2 text-sm text-foreground"
              :value="preferences.ipv6Mode"
              :disabled="ovrenNetBusy"
              @change="
                savePreferences({
                  ...preferences,
                  ipv6Mode: ($event.target as HTMLSelectElement).value as OvrenNetPreferences['ipv6Mode'],
                })
              "
            >
              <option value="auto">{{ t('ovrenNet.ipv6Auto') }}</option>
              <option value="on">{{ t('ovrenNet.ipv6On') }}</option>
              <option value="off">{{ t('ovrenNet.ipv6Off') }}</option>
            </select>
          </label>
          <label class="flex items-center justify-between gap-3 rounded-lg border bg-background p-3 text-sm">
            <span>{{ t('ovrenNet.mtu') }}</span>
            <input
              type="number"
              min="576"
              max="9000"
              step="1"
              class="h-9 w-28 rounded-md border bg-background px-2 text-sm text-foreground"
              :value="preferences.mtu"
              :disabled="ovrenNetBusy"
              @change="savePreferences({ ...preferences, mtu: Number(($event.target as HTMLInputElement).value) })"
            />
          </label>
          <label class="flex items-center gap-3 rounded-lg border bg-background p-3 text-sm md:col-span-2">
            <input
              type="checkbox"
              class="size-4 accent-primary"
              :checked="preferences.startWithOvren"
              :disabled="ovrenNetBusy"
              @change="savePreferences({ ...preferences, startWithOvren: ($event.target as HTMLInputElement).checked })"
            />
            <span>{{ t('ovrenNet.startWithOvren') }}</span>
          </label>
        </div>

        <ul v-if="ovrenNetAdapters.length" class="mb-0 mt-4 grid list-none gap-3 p-0 md:grid-cols-2">
          <li
            v-for="adapter in ovrenNetAdapters"
            :key="`settings-${adapter.name}`"
            class="min-w-0 rounded-lg border bg-background p-4"
          >
            <label class="flex min-w-0 items-center gap-3 text-sm font-medium text-foreground">
              <input
                type="checkbox"
                class="size-4 accent-primary"
                :checked="isAdapterIncluded(adapter.name)"
                :disabled="ovrenNetBusy || (isAdapterIncluded(adapter.name) && selectedAdapterCount <= 1)"
                @change="setIncludedAdapter(adapter.name, ($event.target as HTMLInputElement).checked)"
              />
              <span class="truncate">{{ adapter.name }}</span>
            </label>
            <div class="mt-3 grid grid-cols-2 gap-3">
              <label class="flex flex-col gap-1 text-xs text-muted-foreground">
                {{ t('ovrenNet.adapterWeight') }}
                <input
                  type="number"
                  min="0.05"
                  max="1000"
                  step="0.05"
                  class="h-9 rounded-md border bg-background px-2 text-sm text-foreground"
                  :value="preferences.weights[adapter.name] ?? 1"
                  :disabled="ovrenNetBusy || preferences.autoWeight"
                  @change="setAdapterNumber('weights', adapter.name, $event)"
                />
              </label>
              <label class="flex flex-col gap-1 text-xs text-muted-foreground">
                {{ t('ovrenNet.adapterLimit') }}
                <input
                  type="number"
                  min="0"
                  max="100000"
                  step="1"
                  class="h-9 rounded-md border bg-background px-2 text-sm text-foreground"
                  :value="preferences.limitsMbps[adapter.name] ?? 0"
                  :disabled="ovrenNetBusy"
                  @change="setAdapterNumber('limitsMbps', adapter.name, $event)"
                />
              </label>
            </div>
          </li>
        </ul>
      </section>

      <section class="mt-6 border-t pt-6" :aria-label="t('ovrenNet.proxyTitle')">
        <h3 class="m-0 text-base font-medium text-foreground">{{ t('ovrenNet.proxyTitle') }}</h3>
        <p class="mb-0 mt-1 text-sm text-muted-foreground">{{ t('ovrenNet.proxyDescription') }}</p>
        <div class="mt-3 flex flex-wrap items-center gap-3">
          <code class="rounded-md border bg-background px-3 py-2 text-sm text-foreground">127.0.0.1:1080</code>
          <Button
            v-if="!ovrenNet?.socksRunning"
            variant="outline"
            :disabled="ovrenNetBusy || !preferencesReady || includedAdapterCount === 0"
            @click="runOvrenNetAction('socksStart')"
          >
            {{ t('ovrenNet.proxyStart') }}
          </Button>
          <Button v-else variant="outline" :disabled="ovrenNetBusy" @click="runOvrenNetAction('socksStop')">
            {{ t('ovrenNet.proxyStop') }}
          </Button>
        </div>
      </section>

      <div class="mt-6 flex flex-wrap items-center gap-3 border-t pt-6">
        <Button
          v-if="!ovrenNet?.running"
          variant="outline"
          :disabled="ovrenNetBusy"
          @click="runOvrenNetAction('recover')"
        >
          {{ t('ovrenNet.recover') }}
        </Button>
      </div>
    </section>
  </MdPageShell>
</template>
