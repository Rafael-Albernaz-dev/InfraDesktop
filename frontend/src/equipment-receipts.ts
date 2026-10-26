// UI-only reconciliation IDs. The durable operation ledger remains in the daemon.
// A verified activation switch invalidates the former authority at the daemon;
// transient disconnect and component unmount deliberately do not call this API.
export const pendingStarts = new Map<string, string>();
export const pendingEnrollments = new Map<
  string,
  { id: string; operation_id: string }
>();
export const pendingPPPoE = new Map<
  string,
  { operation: string; reference?: string; revision?: number }
>();
let authority = '';
export function acceptEquipmentAuthority(profile: string, activation: string) {
  const next = [profile, activation].join('|');
  if (next === authority) return;
  pendingStarts.clear();
  pendingEnrollments.clear();
  pendingPPPoE.clear();
  authority = next;
}
