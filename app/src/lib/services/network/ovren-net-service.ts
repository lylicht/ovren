import { invoke } from '@tauri-apps/api/core';
import type { OvrenNetPreferences } from '@/lib/models/network/ovren-net-preferences';

export interface OvrenNetAdapter {
  name: string;
  connected: boolean;
  downloadBytesPerSecond: number;
  uploadBytesPerSecond: number;
  linkDownloadBytesPerSecond: number;
  linkUploadBytesPerSecond: number;
  activeFlows: number;
  totalFlows: number;
  state: string;
  ipv6State: string;
  weight: number;
  weight6: number;
  limitMbps: number;
}

export interface OvrenNetSnapshot {
  running: boolean;
  enabled: boolean;
  pausedReason: string;
  since?: string;
  adapters: OvrenNetAdapter[];
  config: OvrenNetPreferences;
  configError: string;
  socksRunning: boolean;
}

export class OvrenNetService {
  static snapshot(config: OvrenNetPreferences): Promise<OvrenNetSnapshot> {
    return invoke<OvrenNetSnapshot>('ovren_net_snapshot', { config });
  }

  static start(config: OvrenNetPreferences): Promise<void> {
    return invoke('ovren_net_start', { config });
  }

  static configure(config: OvrenNetPreferences): Promise<void> {
    return invoke('ovren_net_configure', { config });
  }

  static pause(): Promise<void> {
    return invoke('ovren_net_pause');
  }

  static resume(): Promise<void> {
    return invoke('ovren_net_resume');
  }

  static stop(): Promise<void> {
    return invoke('ovren_net_stop');
  }

  static recover(): Promise<void> {
    return invoke('ovren_net_recover');
  }

  static socksStart(config: OvrenNetPreferences): Promise<void> {
    return invoke('ovren_net_socks_start', { config });
  }

  static socksStop(): Promise<void> {
    return invoke('ovren_net_socks_stop');
  }
}
