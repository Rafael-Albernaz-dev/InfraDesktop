export type RadioMetric = {
  label?: string;
  name: string;
  value: number;
  unit: string;
  direction: string;
  perspective: string;
  device_id: string;
  interface: string;
  observed_at: string;
  quality: string;
  best?: number;
  worst?: number;
};
export type RadioClient = {
  cpe_id?: string;
  mac: string;
  name: string;
  interface: string;
  bssid: string;
  wireless_uptime?: string;
  last_activity?: string;
  metrics: RadioMetric[];
};
