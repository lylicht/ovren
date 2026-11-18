export type OvrenNetIpv6Mode = 'auto' | 'on' | 'off';

export interface OvrenNetPreferences {
  includedAdapters: string[];
  autoWeight: boolean;
  weights: Record<string, number>;
  limitsMbps: Record<string, number>;
  ipv6Mode: OvrenNetIpv6Mode;
  autoPause: boolean;
  mtu: number;
  startWithOvren: boolean;
}

const MAX_ADAPTERS = 32;
const MAX_ADAPTER_NAME_LENGTH = 48;
const MIN_WEIGHT = 0.05;
const MAX_WEIGHT = 1000;
const MAX_LIMIT_MBPS = 100000;
const MIN_MTU = 576;
const MAX_MTU = 9000;
const PREFERENCE_KEYS = new Set([
  'includedAdapters',
  'autoWeight',
  'weights',
  'limitsMbps',
  'ipv6Mode',
  'autoPause',
  'mtu',
  'startWithOvren',
]);

export function defaults(): OvrenNetPreferences {
  return {
    includedAdapters: [],
    autoWeight: true,
    weights: {},
    limitsMbps: {},
    ipv6Mode: 'auto',
    autoPause: false,
    mtu: MAX_MTU,
    startWithOvren: false,
  };
}

export function parse(value: unknown): OvrenNetPreferences {
  if (!isRecord(value) || Object.keys(value).some(key => !PREFERENCE_KEYS.has(key))) {
    throw new TypeError('Invalid OvrenNET preferences.');
  }

  return {
    includedAdapters: parseAdapterNames(value.includedAdapters),
    autoWeight: parseBoolean(value.autoWeight),
    weights: parseNumberMap(value.weights, MIN_WEIGHT, MAX_WEIGHT),
    limitsMbps: parseNumberMap(value.limitsMbps, 0, MAX_LIMIT_MBPS),
    ipv6Mode: parseIpv6Mode(value.ipv6Mode),
    autoPause: parseBoolean(value.autoPause),
    mtu: parseMtu(value.mtu),
    startWithOvren: parseBoolean(value.startWithOvren),
  };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function parseAdapterNames(value: unknown): string[] {
  if (!Array.isArray(value) || value.length > MAX_ADAPTERS || !value.every(isAdapterName)) {
    throw new TypeError('Invalid OvrenNET adapter selection.');
  }
  if (new Set(value).size !== value.length) throw new TypeError('Duplicate OvrenNET adapter.');
  return [...value];
}

function isAdapterName(value: unknown): value is string {
  return (
    typeof value === 'string' &&
    value.length > 0 &&
    [...value].length <= MAX_ADAPTER_NAME_LENGTH &&
    value.trim() === value &&
    ![...value].some(character => {
      const codePoint = character.codePointAt(0)!;
      return codePoint <= 0x1f || codePoint === 0x7f;
    })
  );
}

function parseNumberMap(value: unknown, min: number, max: number): Record<string, number> {
  if (!isRecord(value) || Object.keys(value).length > MAX_ADAPTERS) {
    throw new TypeError('Invalid OvrenNET adapter values.');
  }
  const result: Record<string, number> = {};
  for (const [name, amount] of Object.entries(value)) {
    if (
      !isAdapterName(name) ||
      typeof amount !== 'number' ||
      !Number.isFinite(amount) ||
      amount < min ||
      amount > max
    ) {
      throw new TypeError('Invalid OvrenNET adapter values.');
    }
    result[name] = amount;
  }
  return result;
}

function parseBoolean(value: unknown): boolean {
  if (typeof value !== 'boolean') throw new TypeError('Invalid OvrenNET preference.');
  return value;
}

function parseIpv6Mode(value: unknown): OvrenNetIpv6Mode {
  if (value === 'auto' || value === 'on' || value === 'off') return value;
  throw new TypeError('Invalid OvrenNET IPv6 mode.');
}

function parseMtu(value: unknown): number {
  if (typeof value !== 'number' || !Number.isInteger(value) || value < MIN_MTU || value > MAX_MTU) {
    throw new TypeError('Invalid OvrenNET MTU.');
  }
  return value;
}
