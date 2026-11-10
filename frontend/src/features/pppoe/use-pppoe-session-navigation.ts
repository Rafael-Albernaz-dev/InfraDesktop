import type { PPRow } from '../../platform/wails/presentation';
import { useContext, useEffect, useRef } from 'react';
import { useOpenLinkedSession } from './linked-session-navigation';
import type { PPPoEProps } from './pppoe';
import type { PPPoERefs, usePPPoEState } from './pppoe-state';

import type {
  SessionNavigation,
  SessionRouteTarget,
} from '../equipment/session-navigation';
import { SessionRoute } from '../equipment/session-navigation';
type State = ReturnType<typeof usePPPoEState>;
function useSessionReturnFocus(
  select: State['select'],
  searchControlRef: PPPoERefs['searchControlRef'],
  active: boolean,
) {
  const wasActive = useRef(false);
  const returnFocus = useRef<{ container: Element; label: string } | undefined>(
    undefined,
  );
  useEffect(() => {
    if (wasActive.current && !active) {
      select(undefined);
      const origin = returnFocus.current;
      queueMicrotask(() => {
        const focused = origin?.container.ownerDocument.activeElement;
        if (
          focused &&
          focused !== origin?.container.ownerDocument.body &&
          focused.isConnected &&
          origin?.container.contains(focused)
        )
          return;
        const button = Array.from(
          origin?.container.querySelectorAll('button') || [],
        ).find(
          (element) => element.getAttribute('aria-label') === origin?.label,
        );
        if (button) button.focus();
        else searchControlRef.current?.focus();
      });
    }
    wasActive.current = active;
  }, [active, select, searchControlRef]);
  const rememberFocus = (element: HTMLElement) => {
    const container = element.closest('.pppoe-workspace')?.parentElement;
    const label = element.getAttribute('aria-label');
    if (container && label) returnFocus.current = { container, label };
  };
  return rememberFocus;
}
function matchesTarget(
  target: SessionRouteTarget | undefined,
  s: State,
  deviceID: string,
) {
  if (!target) return true;
  return (
    target.device === deviceID &&
    target.observation === s.current?.observation_id &&
    target.row === s.selected?.row_id &&
    target.login === s.selected?.login
  );
}
function linkedTarget(
  n: SessionNavigation | undefined,
  matches: boolean,
  fallback: PPPoEProps['sessionTarget'],
): PPPoEProps['sessionTarget'] {
  if (n?.target && !matches)
    return {
      deviceID: n.target.device,
      observation: n.target.observation,
      row: n.target.row,
    };
  if (!n?.active) return fallback;
  return undefined;
}
export function usePPPoESessionNavigation(
  s: State,
  refs: PPPoERefs,
  p: PPPoEProps,
) {
  const navigation = useContext(SessionRoute),
    { device, sessionTarget, onSessionConsumed } = p;
  const active = navigation ? navigation.active : !!s.selected;
  const rememberFocus = useSessionReturnFocus(
    s.select,
    refs.searchControlRef,
    active,
  );
  const openSession = (row: PPRow, originDevice?: string) => {
    if (!s.current) return;
    navigation?.open({
      device: device.id,
      observation: s.current.observation_id,
      row: row.row_id,
      login: row.login,
      ...(originDevice ? { originDevice } : {}),
    });
  };
  const restoredTarget = navigation?.target;
  const selectedMatches = matchesTarget(restoredTarget, s, device.id);
  const targetToLoad = linkedTarget(navigation, selectedMatches, sessionTarget);
  useOpenLinkedSession(s, targetToLoad, (row) => {
    if (
      navigation?.active &&
      navigation.target &&
      (row.row_id !== navigation.target.row ||
        row.login !== navigation.target.login)
    ) {
      s.select(undefined);
      s.setError('A sessão retornada não corresponde ao alvo solicitado.');
      return;
    }
    if (!navigation?.active) {
      onSessionConsumed?.();
      openSession(row, targetToLoad?.originDevice);
    }
  });
  return { navigation, active, selectedMatches, openSession, rememberFocus };
}
