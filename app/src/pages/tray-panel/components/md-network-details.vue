<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import MdConfirmDialog from '@/components/custom/md-confirm-dialog.vue';
import { ResidentService } from '@/lib/services/resident/resident-service';
import { LoggerService } from '@/lib/services/diagnostics/logger-service';
import { useTrayPanelStore } from '@/stores/tray-panel-store';
import { useI18n } from 'vue-i18n';
import type { ResourceReadings } from '@/lib/models/system-resources/system-resources';
import { ByteSizeService } from '@/lib/services/settings/byte-size-service';
import MdTooltip from '@/components/custom/md-tooltip.vue';
import MdResourceOverview from './md-resource-overview.vue';
import MdResourceFacts from './md-resource-facts.vue';
import { OvrenNetService, type OvrenNetSnapshot } from '@/lib/services/network/ovren-net-service';
import { OvrenNetPreferencesService } from '@/lib/services/network/ovren-net-preferences-service';
import type { OvrenNetPreferences } from '@/lib/models/network/ovren-net-preferences';

const props = defineProps<{ reading: ResourceReadings; active: boolean }>();
const { t, n, locale } = useI18n({ useScope: 'global' });
const network = computed(() => (props.reading.network.status === 'ready' ? props.reading.network.value : null));
const ovrenNet = ref<OvrenNetSnapshot | null>(null);
const ovrenNetPreferences = ref<OvrenNetPreferences>(OvrenNetPreferencesService.defaults());
const ovrenNetBusy = ref(false);
const ovrenNetError = ref(false);
const ovrenNetLabel = computed(() => {
  if (!ovrenNet.value?.running) return t('ovrenNet.statusStopped');
  if (ovrenNet.value.pausedReason === 'manual' || ovrenNet.value.pausedReason === 'automatic') {
    return t('ovrenNet.statusPaused');
  }
  return ovrenNet.value.enabled ? t('ovrenNet.statusActive') : t('ovrenNet.statusStarting');
});
const ovrenNetActionLabel = computed(() =>
  !ovrenNet.value?.running ? t('ovrenNet.start') : ovrenNet.value.enabled ? t('ovrenNet.pause') : t('ovrenNet.resume')
);
function speed(bits: number | null | undefined): string {
  if (bits === null || bits === undefined || !Number.isFinite(bits) || bits <= 0) return '—';
  const scale = bits >= 1e9 ? 1e9 : bits >= 1e6 ? 1e6 : 1e3;
  return `${n(Number((bits / scale).toFixed(1)))} ${scale === 1e9 ? 'Gbps' : scale === 1e6 ? 'Mbps' : 'Kbps'}`;
}
const connection = computed(() => {
  const value = network.value;
  if (!value) return '—';
  if (value.interface.kind === 'wifi' && value.receiveLinkBps === null) return speed(value.transmitLinkBps);
  return value.receiveLinkBps === value.transmitLinkBps
    ? speed(value.receiveLinkBps)
    : `↓ ${speed(value.receiveLinkBps)} · ↑ ${speed(value.transmitLinkBps)}`;
});
const cumulative = computed(() => network.value?.cumulative);
const trafficHint = computed(() => {
  const started = cumulative.value?.startedAtMs;
  if (!started) return t('networkDetails.cumulativeUnavailable');
  const since = started
    ? new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(started)
    : '—';
  return t('networkDetails.cumulativeHint', { since });
});
const trafficFacts = computed(() => [
  {
    key: 'received',
    label: t('networkDetails.totalReceived'),
    total: cumulative.value?.receivedBytes,
    session: network.value?.receivedBytesSinceStart,
  },
  {
    key: 'transmitted',
    label: t('networkDetails.totalTransmitted'),
    total: cumulative.value?.transmittedBytes,
    session: network.value?.transmittedBytesSinceStart,
  },
]);
const resetTarget = ref<{ id: string; name: string; startedAtMs: number } | null>(null);
const resetting = ref(false);
const resetError = ref(false);
let ovrenNetTimer: number | null = null;
async function refreshOvrenNet() {
  try {
    ovrenNet.value = await OvrenNetService.snapshot(ovrenNetPreferences.value);
    ovrenNetError.value = ovrenNet.value.configError !== '';
  } catch (cause) {
    ovrenNetError.value = true;
    LoggerService.warn('monitoring', 'ovren_net_status_failed', { error: cause });
  }
}
async function toggleOvrenNet() {
  if (ovrenNetBusy.value) return;
  ovrenNetBusy.value = true;
  ovrenNetError.value = false;
  try {
    if (!ovrenNet.value?.running) await OvrenNetService.start(ovrenNetPreferences.value);
    else if (ovrenNet.value.enabled) await OvrenNetService.pause();
    else await OvrenNetService.resume();
    await refreshOvrenNet();
  } catch (cause) {
    ovrenNetError.value = true;
    LoggerService.warn('monitoring', 'ovren_net_action_failed', { error: cause });
  } finally {
    ovrenNetBusy.value = false;
  }
}
function requestReset() {
  resetError.value = false;
  const value = network.value;
  if (value?.cumulative)
    resetTarget.value = {
      id: value.interface.id,
      name: value.interface.name,
      startedAtMs: value.cumulative.startedAtMs,
    };
}
async function resetTraffic() {
  const target = resetTarget.value;
  if (!target || resetting.value) return;
  resetting.value = true;
  resetError.value = false;
  try {
    const reading = await ResidentService.resetNetworkTraffic(target.id, target.startedAtMs);
    useTrayPanelStore().accept(reading);
    resetTarget.value = null;
  } catch {
    resetError.value = true;
    LoggerService.warn('monitoring', 'network_traffic_reset_failed');
  } finally {
    resetting.value = false;
  }
}
watch(
  () => props.active,
  active => {
    if (!active && !resetting.value) resetTarget.value = null;
    if (active) void refreshOvrenNet();
  }
);
onMounted(() => {
  void OvrenNetPreferencesService.load()
    .then(preferences => {
      ovrenNetPreferences.value = preferences;
      if (props.active) void refreshOvrenNet();
    })
    .catch(cause => {
      ovrenNetError.value = true;
      LoggerService.warn('monitoring', 'ovren_net_preferences_load_failed', { error: cause });
    });
  ovrenNetTimer = window.setInterval(() => {
    if (props.active) void refreshOvrenNet();
  }, 3000);
});
onBeforeUnmount(() => {
  if (ovrenNetTimer !== null) window.clearInterval(ovrenNetTimer);
});
const interfaceFacts = computed(() => {
  const value = network.value?.interface;
  const rows = [
    { key: 'interface', label: t('systemStatus.interface'), value: value?.name ?? '—', unit: '' },
    {
      key: 'status',
      label: t('networkDetails.status'),
      value: value ? t(value.connected ? 'networkDetails.connected' : 'systemStatus.disconnected') : '—',
      unit: '',
    },
    {
      key: 'mac',
      label: t('networkDetails.hardwareAddress'),
      value: value?.details?.hardwareAddress ?? '—',
      unit: '',
      copy: true,
    },
    {
      key: 'ipv4',
      label: 'IPv4',
      value: value?.details?.addresses.filter(address => !address.includes(':')).join(', ') || '—',
      unit: '',
      copy: true,
    },
    {
      key: 'ipv6',
      label: 'IPv6',
      value: value?.details?.addresses.filter(address => address.includes(':')).join(', ') || '—',
      unit: '',
      copy: true,
    },
  ];
  const wifi = value?.details?.wifi;
  if (wifi) {
    rows.push({ key: 'standard', label: t('networkDetails.standard'), value: wifi.standard ?? '—', unit: '' });
    const channel = [
      wifi.bandGhz === null ? '' : `${n(wifi.bandGhz)} GHz`,
      wifi.channelWidthMhz === null ? '' : `${wifi.channelWidthMhz} MHz`,
    ]
      .filter(Boolean)
      .join(', ');
    rows.push({
      key: 'channel',
      label: t('networkDetails.channel'),
      value: wifi.channel === null ? '—' : `${wifi.channel}${channel ? ` (${channel})` : ''}`,
      unit: '',
    });
  }
  return rows;
});
</script>

<template>
  <MdResourceOverview metric="network" :reading="reading" :active="active" class="detail-summary network-summary" />
  <section class="network-details ovren-net-quick" :aria-label="t('ovrenNet.title')">
    <span class="min-w-0 truncate text-xs font-medium text-foreground"
      >{{ t('ovrenNet.title') }} · {{ ovrenNetLabel }}</span
    >
    <button
      type="button"
      class="ovren-net-toggle"
      :disabled="ovrenNetBusy || (!ovrenNet?.running && (ovrenNet?.adapters.length ?? 0) === 0)"
      @click="toggleOvrenNet"
    >
      {{ ovrenNetBusy ? t('ovrenNet.statusStarting') : ovrenNetActionLabel }}
    </button>
    <p v-if="ovrenNetError" class="m-0 w-full text-xs text-destructive-text" role="alert">{{ t('ovrenNet.error') }}</p>
  </section>
  <section class="network-details network-traffic" :aria-label="t('networkDetails.title')">
    <header>
      <MdTooltip :active="active" :text="trafficHint" multiline>
        <h2 tabindex="0">{{ t('networkDetails.title') }}</h2>
      </MdTooltip>
      <MdTooltip :active="active" :text="t('networkDetails.resetHint')" multiline>
        <button type="button" class="traffic-reset" :disabled="!cumulative || resetting" @click="requestReset">
          {{ t('networkDetails.reset') }}
        </button>
      </MdTooltip>
    </header>
    <dl class="traffic-facts">
      <div v-for="fact in trafficFacts" :key="fact.key">
        <dt>{{ fact.label }}</dt>
        <dd>{{ fact.total === undefined ? '—' : ByteSizeService.bytes(Number(fact.total)) }}</dd>
        <MdTooltip :active="active" :text="t('networkDetails.sessionHint')" multiline>
          <small tabindex="0">{{
            t('networkDetails.sessionValue', {
              value: fact.session === undefined ? '—' : ByteSizeService.bytes(fact.session),
            })
          }}</small>
        </MdTooltip>
      </div>
    </dl>
  </section>
  <section class="network-details network-interface" :aria-label="t('systemStatus.interface')">
    <header>
      <h2>{{ t('systemStatus.interface') }}</h2>
      <MdTooltip
        :active="active"
        multiline
        :text="
          t(
            network?.interface.kind === 'wifi' && network.receiveLinkBps === null
              ? 'networkDetails.wifiLinkHint'
              : 'networkDetails.linkHint'
          )
        "
        ><span tabindex="0">{{ connection }}</span></MdTooltip
      >
    </header>
    <MdResourceFacts :active="active" :facts="interfaceFacts" layout="rows" />
  </section>
  <MdConfirmDialog
    v-if="resetTarget"
    presentation="panel"
    :open="true"
    :title="t('networkDetails.resetTitle')"
    :description="t('networkDetails.resetDescription', { name: resetTarget.name })"
    :cancel-label="t('common.cancel')"
    :confirm-label="t('networkDetails.reset')"
    :busy="resetting"
    @update:open="
      open => {
        if (!open) resetTarget = null;
      }
    "
    @confirm="resetTraffic"
  >
    <template v-if="resetError" #default>
      <p role="alert" class="text-xs text-destructive-text">{{ t('networkDetails.resetFailed') }}</p>
    </template>
  </MdConfirmDialog>
</template>

<style scoped>
@reference "@assets/main.css";
.network-details {
  padding: 12px;
  display: flex;
  flex-direction: column;
  flex: none;
  gap: 12px;
}
.network-traffic {
  padding-block: 4px;
  gap: 8px;
}
.ovren-net-quick {
  flex-direction: row;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--border);
}
.traffic-facts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}
.traffic-facts > div {
  @apply rounded-lg bg-muted/40;
  padding: 8px;
  min-width: 0;
}
.traffic-facts dt,
.traffic-facts small {
  @apply text-muted-foreground;
  display: block;
  font-size: 10px;
  overflow-wrap: anywhere;
}
.traffic-facts dd {
  margin-block: 4px;
  font-size: 18px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}
.traffic-reset,
.ovren-net-toggle {
  @apply text-muted-foreground hover:text-foreground;
  font-size: 11px;
  padding: 2px 4px;
  flex: none;
  cursor: pointer;
}
.traffic-reset:disabled,
.ovren-net-toggle:disabled {
  opacity: 0.5;
  cursor: default;
}
.network-interface {
  @apply border-t border-border;
}
header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
h2 {
  font-size: 12px;
  font-weight: 600;
}
header span {
  @apply text-muted-foreground;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}
.network-summary {
  --resource-summary-height: 104px;
  --resource-trend-height: 28px;
}
</style>
