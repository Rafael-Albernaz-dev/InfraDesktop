import { equipmentMessage } from '../equipment/equipment-api';

import { pppoeAPI as api } from '../../platform/wails/presentation';

export function runPPExport(
  kind: 'sanitized' | 'operational',
  p: {
    profile: string;
    activation: string;
    deviceId: string;
    observationId: string;
    base: string;
    tab: string;
    setBusy: (b: boolean) => void;
    setNotice: (s: string) => void;
    setError: (s: string) => void;
    lifeRef: React.MutableRefObject<{ generation: number }>;
    exportFocusRef: React.MutableRefObject<'sanitized' | 'operational' | ''>;
  },
) {
  p.exportFocusRef.current = kind;
  p.setNotice('');
  p.setError('');
  const g = p.lifeRef.current.generation;
  p.setBusy(true);
  const task =
    kind === 'sanitized'
      ? api.export(
          p.profile,
          p.activation,
          p.deviceId,
          p.observationId,
          p.tab === 'compare' ? p.base : '',
        )
      : api.exportOperational(
          p.profile,
          p.activation,
          p.deviceId,
          p.observationId,
        );
  const okMsg =
    kind === 'sanitized'
      ? 'Arquivo sanitizado salvo.'
      : 'Arquivo CSV operacional salvo.';
  void task
    .then((code) => {
      if (g === p.lifeRef.current.generation) {
        p.setNotice(
          code === 'OK'
            ? okMsg
            : code === 'EXPORT_CANCELLED'
              ? ''
              : equipmentMessage(code),
        );
      }
    })
    .catch(() => {
      if (g === p.lifeRef.current.generation)
        p.setError('Falha na exportação.');
    })
    .finally(() => {
      if (g === p.lifeRef.current.generation) p.setBusy(false);
    });
}
