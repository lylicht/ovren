import { createPinia } from 'pinia';
import { createApp } from 'vue';

import App from './App.vue';
import './assets/main.css';
import 'vue-sonner/style.css';
import { i18n } from './i18n';
import { useAppStore } from './stores/app-store';
import { useAnalysisStore } from './stores/analysis-store';
import { useStorageScopeStore } from './stores/storage-scope-store';
import { OvrenNetService } from './lib/services/network/ovren-net-service';
import { OvrenNetPreferencesService } from './lib/services/network/ovren-net-preferences-service';
import { LoggerService } from './lib/services/diagnostics/logger-service';
import { LOG_DOMAINS, LOG_EVENTS } from './lib/models/diagnostics/telemetry';

const app = createApp(App);
const pinia = createPinia();
app.use(pinia);
app.use(i18n);
const isStandaloneFileBrowserWindow = new URLSearchParams(window.location.search).get('window') === 'file-browser';

async function startApplication() {
  const appStore = useAppStore(pinia);
  const startupTasks = [
    appStore.loadSettings(),
    useAnalysisStore(pinia).initializeViewPreferences(),
    useAnalysisStore(pinia).initializeScanMode(),
  ];
  if (isStandaloneFileBrowserWindow) {
    startupTasks.push(appStore.initialize().then(() => useStorageScopeStore(pinia).initialize(appStore.disks)));
    app.mount('#app');
    void Promise.all(startupTasks).catch(error =>
      LoggerService.warn(LOG_DOMAINS.storageScope, LOG_EVENTS.operationFailed, {
        operation: 'standalone_file_browser_startup',
        error,
      })
    );
    return;
  }

  await Promise.all(startupTasks);
  app.mount('#app');
  void startOvrenNetOnLaunch();
}

async function startOvrenNetOnLaunch() {
  try {
    const preferences = await OvrenNetPreferencesService.load();
    if (preferences.startWithOvren) await OvrenNetService.start(preferences);
  } catch (cause) {
    LoggerService.warn('network', 'ovren_net_startup_failed', { error: cause });
  }
}

void startApplication();
