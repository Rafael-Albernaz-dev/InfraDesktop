import type { EquipmentResult } from './equipment-api';
import { equipmentAPI } from './equipment-api';
import { id } from './equipment-reading-format';
import { pendingEnrollments } from './equipment-receipts';
import { fieldForEquipmentCode, fieldMessages } from './equipment-validation';
import type { ImportRow } from '../../platform/wails/presentation';
import { RadioTransports } from '../radio/radio-contract';

import type { EnrollmentProps, EnrollmentState } from './add-enrollment';

type EnrollmentInput = {
  row: ImportRow;
  another: boolean;
  user: string;
  secret: string;
};
function enrollmentRequest(
  s: EnrollmentState,
  { row, user, secret }: EnrollmentInput,
) {
  return {
    id: s.ids.device,
    operation_id: s.ids.operation,
    revision: 0,
    name: row.name.trim(),
    address: row.address.trim(),
    port: Number(row.port),
    role: row.role,
    block: row.block,
    transport: RadioTransports.ssh,
    credential_id: row.credential === '__inline__' ? '' : row.credential,
    user: row.credential === '__inline__' ? user.trim() : '',
    secret: row.credential === '__inline__' ? secret : '',
  };
}
function acceptEnrollment(
  p: EnrollmentProps,
  s: EnrollmentState,
  result: EquipmentResult | undefined,
  input: EnrollmentInput,
) {
  const unknown =
    !result ||
    ['SERVICE_UNAVAILABLE', 'DEADLINE_EXCEEDED', 'REVISION_CONFLICT'].includes(
      result.code,
    );
  if (result && fieldForEquipmentCode[result.code]) {
    const field = fieldForEquipmentCode[result.code];
    p.setErrors({ [field]: fieldMessages[field] });
  }
  if (!unknown) pendingEnrollments.delete(s.key);
  if (!s.alive.current) return;
  s.setNeedsSecret(Boolean(input.secret) && result?.code !== 'OK' && !unknown);
  s.setUncertain(unknown);
  if (result?.code === 'OK') {
    p.reloadCredentials((n) => n + 1);
    s.setRetry(false);
    s.setIDs({ device: id(), operation: id() });
    p.saved(input.another);
  }
}
export async function saveEnrollment(
  p: EnrollmentProps,
  s: EnrollmentState,
  input: EnrollmentInput,
) {
  if (s.lock.current || s.uncertain) return;
  s.lock.current = true;
  pendingEnrollments.set(s.key, {
    id: s.ids.device,
    operation_id: s.ids.operation,
  });
  const request = enrollmentRequest(s, input);
  let result: EquipmentResult | undefined;
  try {
    result = await p.perform(
      () => equipmentAPI.save(p.profile, p.activation, request),
      undefined,
      true,
      true,
    );
  } finally {
    request.secret = '';
  }
  acceptEnrollment(p, s, result, input);
  s.lock.current = false;
}
export async function reconcileEnrollment(
  p: EnrollmentProps,
  s: EnrollmentState,
) {
  if (s.lock.current) return;
  s.lock.current = true;
  s.setChecking(true);
  const result = await p.perform(
    () =>
      equipmentAPI.enrollmentStatus(
        p.profile,
        p.activation,
        s.ids.device,
        s.ids.operation,
      ),
    undefined,
    false,
  );
  if (result?.code === 'OK' && result.data?.operation_state === 'COMMITTED') {
    pendingEnrollments.delete(s.key);
    if (s.alive.current) {
      s.setUncertain(false);
      p.reloadCredentials((n) => n + 1);
      p.saved(false);
    }
  } else if (
    result?.code === 'OK' &&
    result.data?.operation_state === 'NOT_FOUND'
  ) {
    if (s.alive.current) {
      s.setRetry(true);
      s.setUncertain(false);
    }
  }
  if (s.alive.current) s.setChecking(false);
  s.lock.current = false;
}
