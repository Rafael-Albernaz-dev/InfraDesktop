export type PPQuery = {
  state?: '' | 'ONLINE' | 'OFFLINE';
  search: string;
  duplicates_only: boolean;
  sort: string;
  desc: boolean;
  category: string;
};
export type PPInterval = { min: number; max: number; known: boolean };
export type PPContext = {
  association: string;
  service?: string;
  interface?: string;
  vlan?: number;
  ipv6?: boolean;
  delegated_prefix?: string;
  pd_status?: string;
  profile?: string;
  secret_state?: string;
  upload_limit?: string;
  download_limit?: string;
};
export type PPGroup = {
  service: string;
  vlan?: number;
  count: number;
  peak_7d?: number;
  history_limited?: boolean;
};
export type PPObservation = {
  context_omitted?: boolean;
  online?: number;
  offline?: number;
  observation_id: string;
  count: number;
  provenance: { ended_at: string };
  quality: { status: string };
};
export type PPReference = {
  reference_id: string;
  observation_id: string;
  created_at: string;
  count: number;
};
export type PPRate = { min: number; max: number };
export type PPTraffic = {
  points: { t: number; d?: number; u?: number; gap?: boolean }[];
  download_bytes: string;
  upload_bytes: string;
  covered_ns: number;
  covered_min_ns?: number;
  gaps: number;
  overflow?: boolean;
  average_download?: PPRate;
  average_upload?: PPRate;
  peak_download?: PPRate;
  peak_upload?: PPRate;
};
