import { DataBlock } from '../../shared/ui/DataBlock';
import { KeyValues } from '../../shared/ui/KeyValues';
import { Pager } from '../../shared/ui/Pager';
import { readingAge } from '../equipment/equipment-reading-format';
import { Button } from '../../shared/ui/Button';
import { RadioFamilies, RadioRoles } from '../equipment/radio-contract';
import type { RadioInterface, RadioReading } from '../equipment/radio-api';
import { exactNumber, formatRadioFamily } from '../equipment/network-format';
import { Metrics } from './radio-reading';
import {
  createEquipmentPage,
  createFamilyConfirmation,
} from './radio-equipment-state';
import type { RadioEquipmentState } from './radio-equipment-state';
export function RadioUnsupportedNotice({ reading }: { reading: RadioReading }) {
  return (
    <p className="radio-note">
      Driver ausente ({reading.model || 'modelo não identificado'} /{' '}
      {reading.version || 'firmware não identificado'}).{' '}
      {reading.driver_reason || 'Família ou versão sem driver comprovado.'}
    </p>
  );
}
type ReadyEquipment = RadioEquipmentState & { reading: RadioReading };
function RadioThresholdFields({ state }: { state: RadioEquipmentState }) {
  const parents = state.reading?.interfaces.filter((i) => !i.master) || [],
    count = parents.length === 1 ? parents[0].clients : undefined;
  return (
    <KeyValues
      rows={[
        [
          'Detectada',
          `${formatRadioFamily(state.reading?.family)} · ${state.limits?.confirmed ? 'confirmada' : 'aguarda confirmação'}`,
        ],
        [
          'Aviso a partir de',
          <span key="warning">
            <input
              aria-label="Aviso de painel lotado"
              type="number"
              min="1"
              max="511"
              className="small-number"
              value={state.warning}
              onChange={(e) => state.setWarning(Number(e.target.value))}
            />{' '}
            clientes
          </span>,
        ],
        [
          'Crítico a partir de',
          <span key="critical">
            <input
              aria-label="Crítico de painel lotado"
              type="number"
              min="2"
              max="512"
              className="small-number"
              value={state.critical}
              onChange={(e) => state.setCritical(Number(e.target.value))}
            />{' '}
            clientes
          </span>,
        ],
        [
          'Ocupação do painel',
          count === undefined
            ? 'Não informado nesta leitura'
            : `${count} clientes · ${count >= state.critical ? 'crítico' : count >= state.warning ? 'aviso' : 'normal'}`,
        ],
      ]}
    />
  );
}
export function RadioEquipmentLimits({ state }: { state: ReadyEquipment }) {
  const { reading, device, radio, blocked, warning, critical, limits } = state;
  const confirm = createFamilyConfirmation(state);
  if (device.role !== RadioRoles.ap) return null;
  return (
    <DataBlock
      title="Família e painel lotado"
      source="cadastro local"
      data={{
        state: 'ready',
        content: (
          <fieldset className="device-fields" disabled={radio.busy || blocked}>
            {reading.family === RadioFamilies.unsupported ? (
              <p className="radio-note">
                Driver ausente ({reading.model || 'modelo não identificado'} /{' '}
                {reading.version || 'firmware não identificado'}).{' '}
                {reading.driver_reason ||
                  'Família ou versão sem driver comprovado.'}
              </p>
            ) : (
              <>
                <RadioThresholdFields state={state} />
                <p className="muted">
                  Limiares configuráveis · origem: herdado do Atlas.
                </p>
                <div className="actions">
                  <Button
                    type="submit"
                    disabled={
                      warning < 1 ||
                      critical <= warning ||
                      critical > 512 ||
                      !reading.compatible
                    }
                    onClick={() => void confirm()}
                  >
                    {limits?.confirmed
                      ? 'Salvar limites'
                      : 'Confirmar família detectada'}
                  </Button>
                </div>
              </>
            )}
          </fieldset>
        ),
      }}
    />
  );
}
type InterfaceProps = {
  iface: RadioInterface;
  warning: number;
  critical: number;
};
function RadioInterfaceOccupancy({ iface, warning, critical }: InterfaceProps) {
  return (
    <>
      <p>
        Clientes:{' '}
        {iface.clients === undefined
          ? 'Não informado nesta leitura'
          : exactNumber(iface.clients)}
      </p>
      {iface.clients !== undefined && iface.clients >= critical ? (
        <p role="status">Painel lotado · crítico</p>
      ) : iface.clients !== undefined && iface.clients >= warning ? (
        <p role="status">Painel lotado · aviso</p>
      ) : null}
      {iface.clients === 0 && iface.had_clients && (
        <p role="status">Clientes zero: este AP já teve clientes.</p>
      )}
    </>
  );
}

function RadioInterfaceView({
  iface,
  warning,
  critical,
  mode,
}: InterfaceProps & { mode?: string }) {
  return (
    <>
      <KeyValues
        rows={[
          [
            'BSSID',
            `${iface.bssid} · ${iface.band || 'Não informado nesta leitura'}`,
          ],
          ['SSID', 'Não informado nesta leitura'],
          ['Canal observado', iface.channel || 'Não informado nesta leitura'],
          [
            'Largura configurada',
            iface.configured_width || 'Não informado nesta leitura',
          ],
          ['Modo', mode || 'Não informado nesta leitura'],
          [
            'Clientes',
            iface.clients === undefined
              ? 'Não informado nesta leitura'
              : exactNumber(iface.clients),
          ],
          ['Potência', iface.power || 'Não informado nesta leitura'],
        ]}
      />
      <details className="det">
        <summary>Potência por taxa</summary>
        {iface.power_metrics?.length ? (
          <Metrics metrics={iface.power_metrics} />
        ) : (
          <p>Não informado nesta leitura</p>
        )}
      </details>
      {iface.clients !== undefined &&
        (iface.clients >= warning ||
          (iface.clients === 0 && iface.had_clients)) && (
          <RadioInterfaceOccupancy
            iface={iface}
            warning={warning}
            critical={critical}
          />
        )}
    </>
  );
}
export function RadioEquipmentInterfaces({ state }: { state: ReadyEquipment }) {
  if (state.device.role !== RadioRoles.ap) return null;
  return (
    <>
      {state.reading.interfaces.map((iface) => (
        <DataBlock
          key={iface.name}
          title={
            iface.name +
            (iface.master ? ` (AP virtual de ${iface.master})` : '')
          }
          source="SSH"
          age={readingAge(state.reading.observed_at)}
          data={{
            state: 'ready',
            content: (
              <RadioInterfaceView
                iface={iface}
                warning={state.warning}
                critical={state.critical}
                mode={state.reading.mode}
              />
            ),
          }}
        />
      ))}
    </>
  );
}

export function RadioEquipmentPagination({ state }: { state: ReadyEquipment }) {
  const { offset, radio } = state,
    page = createEquipmentPage(state);
  return (
    <>
      <Pager
        page={Math.floor(offset / 8) + 1}
        total={radio.view?.more ? undefined : Math.floor(offset / 8) + 1}
        hasNext={Boolean(radio.view?.more)}
        first={() => void page(0)}
        next={() => void page(radio.view?.next_offset || 0)}
      />
      {offset > 0 && (
        <Button
          disabled={radio.busy}
          onClick={() => void page(Math.max(0, offset - 8))}
        >
          Clientes anteriores
        </Button>
      )}
    </>
  );
}
