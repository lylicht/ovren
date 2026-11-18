import { createPinia } from 'pinia';
import { createApp } from 'vue';

import App from './App.vue';
import './assets/main.css';
import 'vue-sonner/style.css';
import { i18n } from './i18n';
import { useAppStore } from './stores/app-store';
import { useAnalysisStore } from './stores/analysis-store';
import { OvrenNetService } from './lib/services/network/ovren-net-service';
import { OvrenNetPreferencesService } from './lib/services/network/ovren-net-preferences-service';
import { LoggerService } from './lib/services/diagnostics/logger-service';

const app = createApp(App);
const pinia = createPinia();
app.use(pinia);
app.use(i18n);

async function startApplication() {
  await Promise.all([
    useAppStore(pinia).loadSettings(),
    useAnalysisStore(pinia).initializeViewPreferences(),
    useAnalysisStore(pinia).initializeScanMode(),
  ]);
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
