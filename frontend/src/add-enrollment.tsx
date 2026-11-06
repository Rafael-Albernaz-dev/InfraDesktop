import { useEffect, useRef, useState } from 'react';
import { equipmentAPI } from './equipment-api';
import type { EquipmentPerformFn } from './equipment-dialogs';
import type { EquipmentResult } from './equipment-api';
import type { ImportRow } from './presentation';
import { pendingEnrollments } from './equipment-receipts';
import { RadioTransports } from './radio-contract';
import { fieldForEquipmentCode, fieldMessages } from './equipment-validation';
import { id } from './equipment-reading';

export function useAddEnrollment(p: {
  perform: EquipmentPerformFn;
  reloadCredentials: React.Dispatch<React.SetStateAction<number>>;
  continueEnrollment: (result: EquipmentResult) => void;
  setErrors: (errors: Record<string, string>) => void;
  profile: string;
  activation: string;
  saved: (another: boolean) => void;
}) {
  const key = `${p.profile}|${p.activation}`;
  const receipt = pendingEnrollments.get(key);
  const [ids, setIDs] = useState(() => ({
    device: receipt?.id || id(),
    operation: receipt?.operation_id || id(),
  }));
  const [uncertain, setUncertain] = useState(!!receipt);
  const [checking, setChecking] = useState(false);
  const [retry, setRetry] = useState(false);
  const lock = useRef(false);
  const alive = useRef(true);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);
  const changed = () => {
    if (!uncertain && !retry) setIDs((old) => ({ ...old, operation: id() }));
  };
  const save = async (
    row: ImportRow,
    another: boolean,
    user = '',
    secret = '',
  ) => {
    if (lock.current || uncertain) return;
    lock.current = true;
    pendingEnrollments.set(key, {
      id: ids.device,
      operation_id: ids.operation,
    });
    const request = {
      id: ids.device,
      operation_id: ids.operation,
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
    const unknown =
      !result ||
      [
        'SERVICE_UNAVAILABLE',
        'DEADLINE_EXCEEDED',
        'REVISION_CONFLICT',
      ].includes(result.code);
    if (result && fieldForEquipmentCode[result.code]) {
      const field = fieldForEquipmentCode[result.code];
      p.setErrors({ [field]: fieldMessages[field] });
    }
    if (!unknown) pendingEnrollments.delete(key);
    if (alive.current) {
      setUncertain(unknown);
      if (result?.code === 'OK') {
        p.reloadCredentials((n) => n + 1);
        setRetry(false);
        setIDs({ device: id(), operation: id() });
        if (!another) p.continueEnrollment(result);
        p.saved(another);
      }
    }
    lock.current = false;
  };
  const reconcile = async () => {
    if (lock.current) return;
    lock.current = true;
    setChecking(true);
    const result = await p.perform(
      () =>
        equipmentAPI.enrollmentStatus(
          p.profile,
          p.activation,
          ids.device,
          ids.operation,
        ),
      undefined,
      false,
    );
    if (result?.code === 'OK' && result.data?.operation_state === 'COMMITTED') {
      pendingEnrollments.delete(key);
      if (alive.current) {
        setUncertain(false);
        p.reloadCredentials((n) => n + 1);
        p.continueEnrollment(result);
        p.saved(false);
      }
    } else if (
      result?.code === 'OK' &&
      result.data?.operation_state === 'NOT_FOUND'
    ) {
      if (alive.current) {
        setRetry(true);
        setUncertain(false);
      }
    }
    if (alive.current) setChecking(false);
    lock.current = false;
  };
  return { uncertain, checking, retry, changed, save, reconcile };
}
