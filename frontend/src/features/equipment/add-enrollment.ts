import { useEffect, useRef, useState } from 'react';
import { reconcileEnrollment, saveEnrollment } from './add-enrollment-actions';
import type { EquipmentPerformFn } from './equipment-dialogs';
import { id } from './equipment-reading-format';
import { pendingEnrollments } from '../access/equipment-receipts';
import type { ImportRow } from '../../platform/wails/presentation';

export type EnrollmentProps = {
  perform: EquipmentPerformFn;
  reloadCredentials: React.Dispatch<React.SetStateAction<number>>;
  setErrors: (errors: Record<string, string>) => void;
  profile: string;
  activation: string;
  saved: (another: boolean) => void;
};

export function useEnrollmentState(key: string) {
  const receipt = pendingEnrollments.get(key);
  const [ids, setIDs] = useState(() => ({
    device: receipt?.id || id(),
    operation: receipt?.operation_id || id(),
  }));
  const [uncertain, setUncertain] = useState(!!receipt);
  const [checking, setChecking] = useState(false);
  const [retry, setRetry] = useState(false);
  const [needsSecret, setNeedsSecret] = useState(false);
  const lock = useRef(false);
  const alive = useRef(true);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);

  return {
    key,
    ids,
    setIDs,
    uncertain,
    setUncertain,
    checking,
    setChecking,
    retry,
    setRetry,
    needsSecret,
    setNeedsSecret,
    lock,
    alive,
  };
}
export type EnrollmentState = ReturnType<typeof useEnrollmentState>;
export function useAddEnrollment(p: EnrollmentProps) {
  const s = useEnrollmentState(`${p.profile}|${p.activation}`);
  const changed = () => {
    if (!s.uncertain && !s.retry)
      s.setIDs((old) => ({ ...old, operation: id() }));
  };
  return {
    uncertain: s.uncertain,
    checking: s.checking,
    retry: s.retry,
    needsSecret: s.needsSecret,
    changed,
    save: (row: ImportRow, another: boolean, user = '', secret = '') =>
      saveEnrollment(p, s, { row, another, user, secret }),
    reconcile: () => reconcileEnrollment(p, s),
  };
}
