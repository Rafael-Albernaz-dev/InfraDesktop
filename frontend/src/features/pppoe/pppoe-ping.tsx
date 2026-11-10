import type { Device } from '../equipment/equipment-api';
import { useDiagnostic } from './pppoe-diag-state';
import { diagnosticMessage } from './pppoe-diagnostic-message';
import { PingResultDetails } from './pppoe-ping-result';
import { Button } from '../../shared/ui/Button';
type PPPoEPingProps = {
  profile: string;
  activation: string;
  device: Device;
  observation: string;
  row: string;
  blocked: boolean;
  compact?: boolean;
};
type PingViewProps = PPPoEPingProps & {
  view: ReturnType<typeof useDiagnostic>;
};

function pingLabel(view: PingViewProps['view'], compact?: boolean) {
  if (view.starting || view.job) return 'Sondando…';
  if (view.uncertain) return 'Conferir andamento';
  return compact ? 'Ping' : 'Sondar IP pelo concentrador';
}
function PingHeader(p: PingViewProps) {
  const { view, compact } = p;
  return (
    <>
      {!compact && <h3>Ping pelo concentrador</h3>}
      {!compact && (
        <p>
          3 pacotes · intervalo de 200 ms · contexto main, quando comprovado. A
          sessão é revalidada antes da sondagem.
        </p>
      )}
      {compact && !view.loading && !view.error && !view.meta && (
        <p>Ainda sem leitura</p>
      )}
    </>
  );
}
function PingActions(p: PingViewProps) {
  const { view, compact, blocked, device } = p;
  const pingLabelText = pingLabel(view, compact);
  return (
    <>
      <Button
        type="submit"
        disabled={
          blocked ||
          view.loading ||
          view.starting ||
          !!view.job ||
          !device.active ||
          !device.fingerprint
        }
        onClick={() => void view.start()}
      >
        {pingLabelText}
      </Button>
      {view.job && (
        <Button type="submit" onClick={() => void view.cancel()}>
          Cancelar sondagem
        </Button>
      )}
      {view.statusFailed && (
        <Button type="submit" onClick={view.retryStatus}>
          Conferir andamento
        </Button>
      )}
    </>
  );
}
function PingAttempt(p: PingViewProps) {
  const { view, observation, row } = p;
  const isTargetAttempt =
    view.attempt?.source_observation === observation &&
    view.attempt?.source_row === row;
  return (
    <>
      {!view.error &&
        isTargetAttempt &&
        view.attempt?.state !== 'RUNNING' &&
        view.attempt?.code !== 'OK' && (
          <p role="status">
            Tentativa de sondagem incompleta:{' '}
            {diagnosticMessage(view.attempt?.code || '')}{' '}
            {view.meta
              ? 'A última sondagem boa e sua data foram preservadas.'
              : 'Nenhuma sondagem boa disponível.'}
          </p>
        )}
    </>
  );
}
function PingReadStatus(p: PingViewProps) {
  const { view, compact } = p;
  return (
    <>
      {view.error && <p role="alert">{view.error}</p>}
      {view.pageError && <p role="alert">{view.pageError}</p>}
      {view.loading && (
        <p role="status">Carregando última sondagem armazenada…</p>
      )}
      <PingAttempt {...p} />
      {!compact && !view.loading && !view.error && !view.meta && (
        <p>Nenhuma sondagem armazenada para esta sessão.</p>
      )}
      {(compact || view.error || view.pageError) && (
        <Button type="submit" onClick={view.reopen}>
          Reabrir sondagem armazenada
        </Button>
      )}
    </>
  );
}
function PingReadResult(p: PingViewProps) {
  const { view, compact } = p,
    result = view.rows[0]?.ping;
  return (
    <>
      {result && view.meta && (
        <PingResultDetails
          result={result}
          metaDate={view.meta.provenance.ended_at}
        />
      )}
      <p
        className={compact ? 'muted' : undefined}
        title={
          compact
            ? 'A validação e a sondagem ocorrem em uma janela; resposta ICMP não comprova navegação saudável.'
            : undefined
        }
      >
        Não responder não significa estar offline.
        {!compact &&
          ' A validação e a sondagem ocorrem em uma janela; resposta ICMP não comprova navegação saudável.'}
      </p>
    </>
  );
}
export function PPPoEPing(p: PPPoEPingProps) {
  const { profile, activation, device, observation, row } = p;
  const view = useDiagnostic(profile, activation, device, 'ping', {
    observation,
    row,
  });
  const content = { ...p, view };
  return (
    <section
      aria-label="Sondagem da sessão pelo concentrador"
      className="pppoe-diagnostics"
    >
      <PingHeader {...content} />
      <PingActions {...content} />
      <PingReadStatus {...content} />
      <PingReadResult {...content} />
    </section>
  );
}
