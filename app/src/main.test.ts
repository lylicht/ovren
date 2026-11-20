import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => {
  const app = { use: vi.fn(), mount: vi.fn() };
  app.use.mockReturnValue(app);
  return {
    app,
    createApp: vi.fn(() => app),
    pinia: {},
    createPinia: vi.fn(() => ({})),
    appStore: { loadSettings: vi.fn(), initialize: vi.fn(), disks: [] },
    useAppStore: vi.fn(() => mocks.appStore),
    analysisStore: { initializeViewPreferences: vi.fn(), initializeScanMode: vi.fn() },
    useAnalysisStore: vi.fn(() => mocks.analysisStore),
    storageScopeStore: { initialize: vi.fn() },
    useStorageScopeStore: vi.fn(() => mocks.storageScopeStore),
    networkStart: vi.fn(),
    networkPreferencesLoad: vi.fn(),
    loggerWarn: vi.fn(),
  };
});

vi.mock('vue', () => ({ createApp: mocks.createApp }));
vi.mock('pinia', () => ({ createPinia: mocks.createPinia }));
vi.mock('./App.vue', () => ({ default: {} }));
vi.mock('./i18n', () => ({ i18n: {} }));
vi.mock('./stores/app-store', () => ({ useAppStore: mocks.useAppStore }));
vi.mock('./stores/analysis-store', () => ({ useAnalysisStore: mocks.useAnalysisStore }));
vi.mock('./stores/storage-scope-store', () => ({ useStorageScopeStore: mocks.useStorageScopeStore }));
vi.mock('./lib/services/network/ovren-net-service', () => ({ OvrenNetService: { start: mocks.networkStart } }));
vi.mock('./lib/services/network/ovren-net-preferences-service', () => ({
  OvrenNetPreferencesService: { load: mocks.networkPreferencesLoad },
}));
vi.mock('./lib/services/diagnostics/logger-service', () => ({
  LoggerService: { warn: mocks.loggerWarn },
}));

describe('standalone file browser startup', () => {
  beforeEach(() => {
    vi.resetModules();
    vi.clearAllMocks();
    vi.stubGlobal('window', { location: { search: '?window=file-browser' } });
    mocks.app.use.mockReturnValue(mocks.app);
    mocks.appStore.loadSettings.mockReturnValue(new Promise(() => undefined));
    mocks.appStore.initialize.mockReturnValue(new Promise(() => undefined));
    mocks.storageScopeStore.initialize.mockResolvedValue(undefined);
    mocks.analysisStore.initializeViewPreferences.mockResolvedValue(undefined);
    mocks.analysisStore.initializeScanMode.mockResolvedValue(undefined);
  });

  afterEach(() => vi.unstubAllGlobals());

  it('mounts while standalone startup tasks are still pending', async () => {
    await import('./main');
    await new Promise(resolve => setTimeout(resolve, 0));

    expect(mocks.app.mount).toHaveBeenCalledWith('#app');
    expect(mocks.appStore.initialize).toHaveBeenCalledOnce();
  });

  it('keeps the window mounted and logs a storage-scope initialization failure', async () => {
    mocks.appStore.initialize.mockResolvedValue(undefined);
    mocks.storageScopeStore.initialize.mockRejectedValue(new Error('storage scopes unavailable'));

    await import('./main');
    await new Promise(resolve => setTimeout(resolve, 0));

    expect(mocks.app.mount).toHaveBeenCalledWith('#app');
    expect(mocks.loggerWarn).toHaveBeenCalledWith('storage-scope', 'operation_failed', {
      operation: 'standalone_file_browser_startup',
      error: expect.any(Error),
    });
  });
});
