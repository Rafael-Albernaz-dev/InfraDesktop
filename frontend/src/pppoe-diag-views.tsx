import {
  CatalogTable,
  AbsentInterfacesTable,
  RadiusTable,
  RadiusCounterDetails,
  AAAInfo,
} from './pppoe-diagnostic-tables';
import { DataBlock } from './ui/DataBlock';
import { readingAge } from './equipment-reading-format';
import { PingResultDetails } from './pppoe-ping-result';
import { Button } from './ui/Button';
import { Select } from './ui/Select';
import { HeaderAction } from './ui/HeaderAction';
import { useState } from 'react';
import type { Device } from './equipment-api';
import { formatDate } from './network-format';
import { diagnosticMessage, useDiagnostic } from './pppoe-diag-state';
type DiagView = ReturnType<typeof useDiagnostic>;
function DiagnosticListContent(p: {
  view: DiagView;
  scope: 'catalog' | 'radius';
  kind: string;
  setKind: (k: string) => void;
  blocked: boolean;
  device: Device;
}) {
  const { view, scope, kind, setKind, blocked, device } = p;
  if (!view.meta) return null;
  const at = view.meta.provenance.ended_at,
    absent = view.rows.filter((r) => r.not_seen_since);
  const locked =
    blocked ||
    view.loading ||
    view.starting ||
    !device.active ||
    !device.fingerprint;
  const title =
    scope === 'catalog'
      ? 'Serviços, interfaces e VLANs'
      : 'RADIUS no concentrador';
  const content = (
    <>
      <div className="bar">
        {scope === 'catalog' ? (
          <label>
            Filtrar por tipo{' '}
            <Select
              aria-label="Filtrar por tipo"
              value={kind}
              onChange={(e) => {
                setKind(e.target.value);
                view.first();
              }}
            >
              {[
                ['', 'Todos'],
                ['ether', 'Ethernet'],
                ['bridge', 'Bridge'],
                ['vlan', 'VLAN'],
                ['service', 'Serviço PPPoE'],
                ['wlan', 'Wireless'],
              ].map(([v, t]) => (
                <option key={v} value={v}>
                  {t}
                </option>
              ))}
            </Select>
          </label>
        ) : (
          <span className="muted">
            Contadores acumulados desde o boot, por servidor.
          </span>
        )}
        <details className="det">
          <summary>
            {scope === 'radius' ? 'Contadores' : 'Origem da leitura'}
          </summary>
          <p>
            Última leitura boa: {formatDate(at)} · RouterOS{' '}
            {view.meta.provenance.version} · {view.meta.count} itens · origem:
            concentrador via SSH.
          </p>
          {scope === 'radius' && (
            <>
              <p>
                Contadores não identificam motivo de rejeição ou autenticação de
                um assinante. Nenhum log bruto é coletado.
              </p>
              <RadiusCounterDetails rows={view.rows} />
            </>
          )}
        </details>
        <span className="grow" />
        <Button
          disabled={locked}
          onClick={() => (view.job ? void view.cancel() : void view.start())}
        >
          {view.job ? 'Parar' : 'Atualizar'}
        </Button>
        {!(view.error || view.pageError) && (
          <Button onClick={view.reopen}>Reabrir dados armazenados</Button>
        )}
      </div>
      {view.pageLoading && <p role="status">Carregando página armazenada…</p>}
      {scope === 'catalog' ? (
        <CatalogTable rows={view.rows} />
      ) : (
        <RadiusTable rows={view.rows} />
      )}
      {!view.pageLoading && !view.pageError && !view.rows.length && (
        <p>Nenhum item configurado nesta leitura completa.</p>
      )}
      {scope === 'radius' &&
        view.meta.count === 1 &&
        view.rows[0]?.kind === 'aaa' && (
          <p>Nenhum servidor RADIUS configurado nesta leitura.</p>
        )}
      <div className="pager">
        <Button
          aria-label="Início do diagnóstico"
          disabled={!view.cursor}
          onClick={view.first}
        >
          Início
        </Button>
        <span>
          Página {view.cursor ? 2 : 1}
          {!view.next ? ' de 1' : ''}
        </span>
        <Button
          aria-label="Próxima página do diagnóstico"
          disabled={view.pageLoading || !!view.pageError || !view.next}
          onClick={view.more}
        >
          Próxima
        </Button>
      </div>
      {scope === 'catalog' && (
        <p className="muted">
          VLANs e faixas representam configuração. Não comprovam vínculo
          individual das sessões.
        </p>
      )}
    </>
  );
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
export function PPPoEDiagnostics({
  profile,
  activation,
  device,
  scope,
  blocked,
}: {
  profile: string;
  activation: string;
  device: Device;
  scope: 'catalog' | 'radius';
  blocked: boolean;
}) {
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
  const actionLabel =
    view.starting || view.job
      ? 'Consultando…'
      : view.uncertain
        ? 'Conferir andamento'
        : scope === 'catalog'
          ? 'Atualizar serviços e VLANs'
          : 'Atualizar RADIUS';
  return (
    <section
      aria-label={title}
      className="pppoe-diagnostics"
      aria-busy={view.loading || view.pageLoading}
    >
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

export function PPPoEPing({
  profile,
  activation,
  device,
  observation,
  row,
  blocked,
}: {
  profile: string;
  activation: string;
  device: Device;
  observation: string;
  row: string;
  blocked: boolean;
}) {
  const view = useDiagnostic(profile, activation, device, 'ping', {
    observation,
    row,
  });
  const result = view.rows[0]?.ping;
  const isTargetAttempt =
    view.attempt?.source_observation === observation &&
    view.attempt?.source_row === row;
  const pingLabel =
    view.starting || view.job
      ? 'Sondando…'
      : view.uncertain
        ? 'Conferir andamento'
        : 'Sondar IP pelo concentrador';
  return (
    <section
      aria-label="Sondagem da sessão pelo concentrador"
      className="pppoe-diagnostics"
    >
      <h3>Ping pelo concentrador</h3>
      <p>
        3 pacotes · intervalo de 200 ms · contexto main, quando comprovado. A
        sessão é revalidada antes da sondagem.
      </p>
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
        {pingLabel}
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
      {view.error && <p role="alert">{view.error}</p>}
      {view.pageError && <p role="alert">{view.pageError}</p>}
      {view.loading && (
        <p role="status">Carregando última sondagem armazenada…</p>
      )}
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
      {!view.loading && !view.error && !view.meta && (
        <p>Nenhuma sondagem armazenada para esta sessão.</p>
      )}
      {(view.error || view.pageError) && (
        <Button type="submit" onClick={view.reopen}>
          Reabrir sondagem armazenada
        </Button>
      )}
      {result && view.meta && (
        <PingResultDetails
          result={result}
          metaDate={view.meta.provenance.ended_at}
        />
      )}
      <p>
        Não responder não significa estar offline. A validação e a sondagem
        ocorrem em uma janela; resposta ICMP não comprova navegação saudável.
      </p>
    </section>
  );
}
