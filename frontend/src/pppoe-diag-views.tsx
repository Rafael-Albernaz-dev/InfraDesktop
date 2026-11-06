import { useState } from 'react';
import type { Device } from './equipment-api';
import { readingAge } from './equipment-reading-format';
import { useDiagnostic } from './pppoe-diag-state';
import { diagnosticMessage } from './pppoe-diagnostic-message';
import { DiagnosticStoredRows } from './pppoe-diagnostic-content';
import { AAAInfo, AbsentInterfacesTable } from './pppoe-diagnostic-tables';
import { Button } from './ui/Button';
import { DataBlock } from './ui/DataBlock';
import { HeaderAction } from './ui/HeaderAction';
type DiagView = ReturnType<typeof useDiagnostic>;
export type DiagnosticListContentProps = {
  view: DiagView;
  scope: 'catalog' | 'radius';
  kind: string;
  setKind: (k: string) => void;
  blocked: boolean;
  device: Device;
};
function DiagnosticListContent(p: DiagnosticListContentProps) {
  const { view, scope } = p;
  if (!view.meta) return null;
  const at = view.meta.provenance.ended_at,
    absent = view.rows.filter((r) => r.not_seen_since);
  const title =
    scope === 'catalog'
      ? 'Serviços, interfaces e VLANs'
      : 'RADIUS no concentrador';
  const content = <DiagnosticStoredRows {...p} />;
  return (
    <>
      {scope === 'radius' && (
        <>
          <div className="grid">
            <DataBlock
              title="PPP AAA"
              source="SSH"
              age={readingAge(at)}
              data={{
                state: 'ready',
                content: (
                  <AAAInfo
                    row={view.rows.find((r) => r.kind === 'aaa')}
                    at={at}
                  />
                ),
              }}
            />
          </div>
          <div className="sp10" />
        </>
      )}
      <DataBlock
        title={title}
        source="SSH"
        age={readingAge(at)}
        data={{ state: view.pageLoading ? 'refreshing' : 'ready', content }}
      />
      {scope === 'catalog' && (
        <>
          <div className="sp10" />
          <DataBlock
            title="Não vistas na leitura atual"
            source="SSH"
            age={readingAge(at)}
            data={{
              state: 'ready',
              content: (
                <>
                  <AbsentInterfacesTable rows={absent} />
                  <p className="muted">
                    Retidas por até 24 h, separadas das presentes.
                  </p>
                  {view.meta.absences_limited && (
                    <p role="status">
                      Retenção de ausentes limitada para preservar a leitura
                      atual.
                    </p>
                  )}
                </>
              ),
            }}
          />
        </>
      )}
    </>
  );
}
type PPPoEDiagnosticsProps = {
  profile: string;
  activation: string;
  device: Device;
  scope: 'catalog' | 'radius';
  blocked: boolean;
};
export function PPPoEDiagnostics({
  profile,
  activation,
  device,
  scope,
  blocked,
}: PPPoEDiagnosticsProps) {
  const [kind, setKind] = useState('');
  const view = useDiagnostic(
    profile,
    activation,
    device,
    scope,
    undefined,
    kind,
  );
  const title =
    scope === 'catalog'
      ? 'Serviços, interfaces e VLANs'
      : 'RADIUS no concentrador';
  return (
    <section
      aria-label={title}
      className="pppoe-diagnostics"
      aria-busy={view.loading || view.pageLoading}
    >
      <DiagnosticHeader
        profile={profile}
        activation={activation}
        device={device}
        scope={scope}
        blocked={blocked}
        view={view}
      />
      <DiagnosticLoadStatus
        profile={profile}
        activation={activation}
        device={device}
        scope={scope}
        blocked={blocked}
        view={view}
      />
      <DiagnosticListContent
        view={view}
        scope={scope}
        kind={kind}
        setKind={setKind}
        blocked={blocked}
        device={device}
      />
      {view.pageError && <p role="alert">{view.pageError}</p>}
      {(view.error || view.pageError) && (
        <Button type="submit" onClick={view.reopen}>
          Reabrir dados armazenados
        </Button>
      )}
    </section>
  );
}

type DiagnosticActionProps = PPPoEDiagnosticsProps & { view: DiagView };
function diagnosticActionLabel(view: DiagView, scope: 'catalog' | 'radius') {
  if (view.starting || view.job) return 'Consultando…';
  if (view.uncertain) return 'Conferir andamento';
  return scope === 'catalog'
    ? 'Atualizar serviços e VLANs'
    : 'Atualizar RADIUS';
}
function DiagnosticHeader(p: DiagnosticActionProps) {
  const { view, blocked, device, scope } = p;
  const actionLabel = diagnosticActionLabel(view, scope);
  return (
    <HeaderAction>
      <Button
        type="submit"
        className="primary"
        aria-label={view.job ? 'Parar' : actionLabel}
        disabled={
          blocked ||
          view.loading ||
          view.starting ||
          !device.active ||
          !device.fingerprint
        }
        onClick={() => (view.job ? void view.cancel() : void view.start())}
      >
        {view.job ? 'Parar' : 'Atualizar'}
      </Button>
    </HeaderAction>
  );
}
function DiagnosticAttempt({ view }: DiagnosticActionProps) {
  return (
    <>
      {!view.error &&
        view.attempt &&
        view.attempt.state !== 'RUNNING' &&
        view.attempt.code !== 'OK' && (
          <p role="status">
            Tentativa{' '}
            {view.attempt.quality?.status === 'limited'
              ? 'limitada'
              : 'incompleta'}
            : {diagnosticMessage(view.attempt.code)}{' '}
            {view.meta
              ? 'A última leitura boa e sua data foram preservadas.'
              : 'Nenhuma leitura boa disponível.'}
          </p>
        )}
    </>
  );
}
function DiagnosticLoadStatus(p: DiagnosticActionProps) {
  const { view } = p;
  return (
    <>
      {view.statusFailed && (
        <Button type="submit" onClick={view.retryStatus}>
          Conferir andamento
        </Button>
      )}
      {view.error && <p role="alert">{view.error}</p>}
      {view.loading && <p role="status">Carregando diagnóstico armazenado…</p>}
      {!view.loading && !view.error && !view.meta && (
        <p>
          Nenhuma leitura deste diagnóstico disponível. Atualize para consultar
          o concentrador.
        </p>
      )}
      <DiagnosticAttempt {...p} />
    </>
  );
}
