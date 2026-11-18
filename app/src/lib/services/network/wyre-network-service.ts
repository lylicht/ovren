import { invoke } from '@tauri-apps/api/core';

export interface NetworkMeasurement {
  latencyMs: number;
  jitterMs: number;
  downloadMbps: number;
  uploadMbps: number;
  failedProbesPercent: number;
  dataMb: number;
  serverHost: string;
  serverLocation: string;
}

export class WyreNetworkService {
  static measure(): Promise<NetworkMeasurement> {
    return invoke<NetworkMeasurement>('network_test');
  }
}
