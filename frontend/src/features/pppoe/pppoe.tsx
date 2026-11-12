import { useSessionsTabRefresh } from './sessions-tab-refresh';
import type { Device } from '../equipment/equipment-api';
import type { LinkedSessionTarget } from './linked-session-navigation';
import { PPPoESessionDetail } from './pppoe-session-screen';
import { usePPPoEState } from './pppoe-state';
import { PPPoEWorkspace } from './pppoe-workspace';
import { SessionLoading } from './pppoe-session-loading';
import { usePPPoERefs } from './use-pppoe-refs';
import { usePPPoESessionNavigation } from './use-pppoe-session-navigation';
export type PPPoEProps = {
  profile: string;
  activation: string;
  device: Device;
  back: () => void;
  section?: 'sessions' | 'services';
  refreshAvailable?: boolean;
  sessionTarget?: LinkedSessionTarget;
  onSessionConsumed?: () => void;
  openEquipment?: (id: string) => void;
};
export function PPPoE(p: PPPoEProps) {
  const { profile, activation, device, openEquipment } = p;
  const section = p.section || 'sessions';
  const refs = usePPPoERefs(),
    { detailHeadingRef, detailTriggerRef, detailIntentRef } = refs;
  const s = usePPPoEState(
    profile,
    activation,
    device,
    refs,
    section === 'services' ? 'catalog' : 'sessions',
  );
  const { navigation, active, selectedMatches, openSession, rememberFocus } =
    usePPPoESessionNavigation(s, refs, p);
  useSessionsTabRefresh(s, p.refreshAvailable || false);
  const disabled = s.busy || Boolean(s.job);
  if (active && !selectedMatches) {
    return (
      <SessionLoading
        tab={s.detailTab}
        onTab={s.setDetailTab}
        login={navigation?.target?.login}
        device={device.name}
        error={s.error}
        back={navigation.back}
      />
    );
  }
  if (active && s.selected && s.current) {
    return (
      <PPPoESessionDetail
        {...s}
        selected={s.selected}
        current={s.current}
        disabled={disabled}
        openEquipment={(id) => {
          navigation?.leave();
          openEquipment?.(id);
        }}
        detailHeadingRef={detailHeadingRef}
        detailTriggerRef={detailTriggerRef}
        detailIntentRef={detailIntentRef}
        back={navigation.back}
      />
    );
  }
  return (
    <PPPoEWorkspace
      s={s}
      refs={refs}
      disabled={disabled}
      section={section}
      openEquipment={openEquipment}
      onOpenSession={openSession}
      onFocusOrigin={rememberFocus}
    />
  );
}
