import { defaults, parse, type OvrenNetPreferences } from '@/lib/models/network/ovren-net-preferences';
import { PreferenceStorageService } from '@/lib/services/settings/preference-storage-service';

export class OvrenNetPreferencesService {
  static defaults(): OvrenNetPreferences {
    return defaults();
  }

  static parse(value: unknown): OvrenNetPreferences {
    return parse(value);
  }

  static async load(): Promise<OvrenNetPreferences> {
    const saved = await PreferenceStorageService.loadOvrenNetPreferences();
    return saved === null ? defaults() : parse(saved);
  }

  static save(preferences: OvrenNetPreferences): Promise<void> {
    return PreferenceStorageService.saveOvrenNetPreferences(parse(preferences));
  }
}
