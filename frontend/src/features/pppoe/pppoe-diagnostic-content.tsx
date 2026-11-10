import { formatDate } from '../equipment/network-format';
import type { DiagnosticListContentProps } from './pppoe-diag-views';
import {
  CatalogTable,
  RadiusCounterDetails,
  RadiusTable,
} from './pppoe-diagnostic-tables';
import { Button } from '../../shared/ui/Button';
import { Select } from '../../shared/ui/Select';
function DiagnosticFilter({
  view,
  scope,
  kind,
  setKind,
}: DiagnosticListContentProps) {
  return (
    <>
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
    </>
  );
}
function DiagnosticProvenance({ view, scope }: DiagnosticListContentProps) {
  if (!view.meta) return null;
  const at = view.meta.provenance.ended_at;
  return (
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
            Contadores não identificam motivo de rejeição ou autenticação de um
            assinante. Nenhum log bruto é coletado.
          </p>
          <RadiusCounterDetails rows={view.rows} />
        </>
      )}
    </details>
  );
}
function DiagnosticToolbar(p: DiagnosticListContentProps) {
  const { view, blocked, device } = p;
  const locked =
    blocked ||
    view.loading ||
    view.starting ||
    !device.active ||
    !device.fingerprint;
  return (
    <div className="bar">
      <DiagnosticFilter {...p} />
      <DiagnosticProvenance {...p} />
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
  );
}
function DiagnosticPager({ view }: DiagnosticListContentProps) {
  return (
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
  );
}
export function DiagnosticStoredRows(p: DiagnosticListContentProps) {
  const { view, scope } = p;
  if (!view.meta) return null;
  return (
    <>
      <DiagnosticToolbar {...p} />
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
      <DiagnosticPager {...p} />
      {scope === 'catalog' && (
        <p className="muted">
          VLANs e faixas representam configuração. Não comprovam vínculo
          individual das sessões.
        </p>
      )}
    </>
  );
}
