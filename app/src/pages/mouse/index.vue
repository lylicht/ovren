<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { Button } from '@/components/ui/button';
import { Dialog, DialogDescription, DialogTitle } from '@/components/ui/dialog';
import MdDialogContent from '@/components/custom/md-dialog-content.vue';
import MdDialogFooter from '@/components/custom/md-dialog-footer.vue';
import MdDialogHeader from '@/components/custom/md-dialog-header.vue';
import MdPageShell from '@/components/custom/md-page-shell.vue';
import { LoggerService } from '@/lib/services/diagnostics/logger-service';
import { WyreMouseService, type WyreMouseSettings } from '@/lib/services/system-settings/wyre-mouse-service';

const { t } = useI18n({ useScope: 'global' });
const current = ref<WyreMouseSettings | null>(null);
const speed = ref(10);
const acceleration = ref(true);
const primaryButton = ref<'left' | 'right'>('left');
const scrollLines = ref(3);
const naturalScroll = ref(false);
const doubleClickMs = ref(500);
const scrollOptions = computed(() => {
  const options = [0, 1, 2, 3, 4, 5, 10, 20, 50, 100, -1];
  if (!options.includes(scrollLines.value)) options.push(scrollLines.value);
  return options.sort((a, b) => (a === -1 ? 1 : b === -1 ? -1 : a - b));
});
const pending = ref(false);
const busy = ref(false);
const error = ref('');
const notice = ref('');
const undo = ref<WyreMouseSettings | null>(null);
const supported = computed(() => current.value?.supported === true);
const changed = computed(() => {
  const settings = current.value;
  return (
    settings?.supported === true &&
    (speed.value !== settings.speed ||
      acceleration.value !== settings.acceleration ||
      (settings.primaryButton !== null && primaryButton.value !== settings.primaryButton) ||
      (settings.scrollLines !== null && scrollLines.value !== settings.scrollLines) ||
      (settings.naturalScroll !== null && naturalScroll.value !== settings.naturalScroll) ||
      (settings.doubleClickMs !== null && doubleClickMs.value !== settings.doubleClickMs))
  );
});

function setSettings(settings: WyreMouseSettings) {
  current.value = settings;
  if (settings.speed !== null) speed.value = settings.speed;
  if (settings.acceleration !== null) acceleration.value = settings.acceleration;
  if (settings.primaryButton !== null) primaryButton.value = settings.primaryButton;
  if (settings.scrollLines !== null) scrollLines.value = settings.scrollLines;
  if (settings.naturalScroll !== null) naturalScroll.value = settings.naturalScroll;
  if (settings.doubleClickMs !== null) doubleClickMs.value = settings.doubleClickMs;
}

async function load() {
  busy.value = true;
  error.value = '';
  undo.value = null;
  try {
    setSettings(await WyreMouseService.read());
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : String(cause);
    LoggerService.warn('system-settings', 'pointer_settings_read_failed', { error: cause });
  } finally {
    busy.value = false;
  }
}

async function apply() {
  if (!current.value || !changed.value || busy.value) return;
  const previous = current.value;
  busy.value = true;
  pending.value = false;
  error.value = '';
  notice.value = '';
  try {
    setSettings(
      await WyreMouseService.apply({
        ...previous,
        speed: speed.value,
        acceleration: acceleration.value,
        accelerationProfile: previous.acceleration === acceleration.value ? previous.accelerationProfile : null,
        primaryButton: previous.primaryButton === null ? null : primaryButton.value,
        scrollLines: previous.scrollLines === null ? null : scrollLines.value,
        naturalScroll: previous.naturalScroll === null ? null : naturalScroll.value,
        doubleClickMs: previous.doubleClickMs === null ? null : doubleClickMs.value,
      })
    );
    undo.value = previous;
    notice.value = t('wyre.mouseApplied');
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : String(cause);
    LoggerService.warn('system-settings', 'pointer_settings_apply_failed', { error: cause });
  } finally {
    busy.value = false;
  }
}

async function restore() {
  if (!undo.value || busy.value) return;
  busy.value = true;
  error.value = '';
  notice.value = '';
  try {
    setSettings(await WyreMouseService.apply(undo.value));
    undo.value = null;
    notice.value = t('wyre.mouseRestored');
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : String(cause);
    LoggerService.warn('system-settings', 'pointer_settings_restore_failed', { error: cause });
  } finally {
    busy.value = false;
  }
}

onMounted(() => void load());
</script>

<template>
  <MdPageShell :title="t('wyre.mouseTitle')" :subtitle="t('wyre.mouseDescription')" content-mode="workspace">
    <div class="grid h-full content-start gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(280px,.65fr)]">
      <section class="rounded-xl border bg-card p-5 sm:p-7">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h2 class="m-0 text-base font-medium text-foreground">{{ t('wyre.pointerSpeed') }}</h2>
            <p class="mt-1 mb-0 text-sm text-muted-foreground">{{ t('wyre.pointerSpeedDescription') }}</p>
          </div>
          <output class="rounded-md border px-3 py-1.5 font-mono text-sm text-foreground" for="pointer-speed"
            >{{ speed }}/20</output
          >
        </div>
        <input
          id="pointer-speed"
          v-model.number="speed"
          class="mt-6 w-full accent-primary"
          type="range"
          min="1"
          max="20"
          step="1"
          :disabled="!supported || busy"
          :aria-label="t('wyre.pointerSpeed')"
        />
        <div class="mt-6 flex items-center justify-between gap-6 border-t pt-5">
          <div>
            <label for="pointer-acceleration" class="text-sm font-medium text-foreground">{{
              t('wyre.pointerAcceleration')
            }}</label>
            <p class="mt-1 mb-0 text-sm text-muted-foreground">{{ t('wyre.pointerAccelerationDescription') }}</p>
          </div>
          <input
            id="pointer-acceleration"
            v-model="acceleration"
            type="checkbox"
            role="switch"
            class="size-5 accent-primary"
            :disabled="!supported || busy"
          />
        </div>
        <div class="mt-6 grid gap-5 border-t pt-5 sm:grid-cols-2">
          <div v-if="current && current.primaryButton !== null">
            <h3 class="m-0 text-sm font-medium text-foreground">{{ t('wyre.primaryButton') }}</h3>
            <div class="mt-3 flex gap-2">
              <Button
                type="button"
                :variant="primaryButton === 'left' ? 'default' : 'outline'"
                :aria-pressed="primaryButton === 'left'"
                :disabled="busy"
                @click="primaryButton = 'left'"
                >{{ t('wyre.primaryLeft') }}</Button
              >
              <Button
                type="button"
                :variant="primaryButton === 'right' ? 'default' : 'outline'"
                :aria-pressed="primaryButton === 'right'"
                :disabled="busy"
                @click="primaryButton = 'right'"
                >{{ t('wyre.primaryRight') }}</Button
              >
            </div>
          </div>
          <div v-if="current && current.naturalScroll !== null" class="flex items-center justify-between gap-4">
            <label for="natural-scroll" class="text-sm font-medium text-foreground">{{
              t('wyre.naturalScroll')
            }}</label>
            <input
              id="natural-scroll"
              v-model="naturalScroll"
              type="checkbox"
              role="switch"
              class="size-5 accent-primary"
              :disabled="busy"
            />
          </div>
          <div v-if="current && current.scrollLines !== null">
            <label for="wheel-lines" class="text-sm font-medium text-foreground">{{
              t('wyre.wheelScrollLines')
            }}</label>
            <select
              id="wheel-lines"
              v-model.number="scrollLines"
              class="mt-3 h-10 w-full rounded-md border bg-background px-3 text-sm text-foreground"
              :disabled="busy"
            >
              <option v-for="lines in scrollOptions" :key="lines" :value="lines">
                {{
                  lines === -1
                    ? t('wyre.wheelPage')
                    : lines === 0
                      ? t('wyre.wheelOff')
                      : t('wyre.wheelLines', { count: lines })
                }}
              </option>
            </select>
          </div>
          <div v-if="current && current.doubleClickMs !== null">
            <label for="double-click-ms" class="text-sm font-medium text-foreground">{{
              t('wyre.doubleClickTime')
            }}</label>
            <div class="mt-3 flex items-center gap-2">
              <input
                id="double-click-ms"
                v-model.number="doubleClickMs"
                class="h-10 w-full rounded-md border bg-background px-3 text-sm text-foreground"
                type="number"
                min="0"
                max="5000"
                step="25"
                :disabled="busy"
              />
              <span class="text-sm text-muted-foreground">ms</span>
            </div>
          </div>
        </div>
        <div class="mt-6 flex flex-wrap items-center gap-3">
          <Button :disabled="!changed || busy" @click="pending = true">{{ t('wyre.reviewMouseChanges') }}</Button>
          <Button v-if="undo" variant="outline" :disabled="busy" @click="restore">{{
            t('wyre.undoMouseChanges')
          }}</Button>
        </div>
        <p v-if="notice" class="mb-0 mt-4 text-sm text-primary-text" role="status">{{ notice }}</p>
        <p v-if="error" class="mb-0 mt-4 text-sm text-destructive-text" role="alert">{{ error }}</p>
      </section>
      <aside class="rounded-xl border bg-card p-5 sm:p-7">
        <h2 class="mt-0 text-base font-medium text-foreground">{{ t('wyre.mouseAvailabilityTitle') }}</h2>
        <p class="mb-0 text-sm leading-relaxed text-muted-foreground">
          {{ supported ? t('wyre.mouseAvailability') : (current?.reason ?? t('wyre.mouseLoading')) }}
        </p>
        <Button class="mt-5" variant="outline" :disabled="busy" @click="load">{{ t('common.retry') }}</Button>
      </aside>
    </div>
    <Dialog v-model:open="pending">
      <MdDialogContent size="compact">
        <MdDialogHeader>
          <DialogTitle>{{ t('wyre.mouseReviewTitle') }}</DialogTitle>
          <DialogDescription>{{ t('wyre.mouseReviewDescription') }}</DialogDescription>
        </MdDialogHeader>
        <MdDialogFooter>
          <Button variant="outline" :disabled="busy" @click="pending = false">{{ t('common.cancel') }}</Button>
          <Button :disabled="busy" @click="apply">{{ t('wyre.confirmMouseChanges') }}</Button>
        </MdDialogFooter>
      </MdDialogContent>
    </Dialog>
  </MdPageShell>
</template>
