import { formatPPUptime } from '../equipment/pppoe-format';
import type { PPPoESessionDetailProps } from './pppoe-session-props';
import { Button } from '../../shared/ui/Button';
export function SessionHeader(p: PPPoESessionDetailProps) {
  const { selected, device, detailIntentRef, select, detailHeadingRef } = p;
  return (
    <div className="head">
      <div className="grow">
        <h1 tabIndex={-1} ref={detailHeadingRef}>
          {selected.login}
        </h1>
        <div className="meta">
          Sessão PPPoE · {device.name} · {selected.state || 'ONLINE'} · IP{' '}
          {selected.address || 'Não informado nesta leitura'}
          {selected.context?.profile && ` · ${selected.context.profile}`}
          {selected.state !== 'OFFLINE' &&
            ` · conectado há ${formatPPUptime(selected.uptime)}`}
        </div>
      </div>
      <Button
        onClick={() => {
          ++detailIntentRef.current;
          select(undefined);
          p.back();
        }}
      >
        Voltar
      </Button>
    </div>
  );
}
