import { invoke } from '@tauri-apps/api/core';

export interface OvrenNetAdapter {
  name: string;
  connected: boolean;
  downloadBytesPerSecond: number;
  uploadBytesPerSecond: number;
  activeFlows: number;
  totalFlows: number;
}

export interface OvrenNetSnapshot {
  running: boolean;
  enabled: boolean;
  since?: string;
  adapters: OvrenNetAdapter[];
}

export class OvrenNetService {
  static snapshot(): Promise<OvrenNetSnapshot> {
    return invoke<OvrenNetSnapshot>('ovren_net_snapshot');
  }

  static start(): Promise<void> {
    return invoke('ovren_net_start');
  }

  static stop(): Promise<void> {
    return invoke('ovren_net_stop');
  }
}
