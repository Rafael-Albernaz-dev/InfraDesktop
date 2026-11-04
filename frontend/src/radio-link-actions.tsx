import { Button } from './ui/Button';

import type { RadioLinkState } from './radio-link-state';
import { RadioModes } from './radio-contract';
import { radioCostLabel } from './radio-cost-label';

export function RadioLinkRunActions({ state }: { state: RadioLinkState }) {
  const { blocked, radio, editing, link, cost } = state;
  return (
    <>
      {radio.busy ? (
        <Button
          type="submit"
          className="primary"
          onClick={() => void radio.stop()}
        >
          Parar
        </Button>
      ) : (
        link &&
        !editing && (
          <div className="radio-actions">
            <Button
              type="submit"
              className="primary"
              disabled={blocked}
              onClick={() => void radio.start(RadioModes.once)}
            >
              Atualizar
            </Button>
            <Button
              type="submit"
              disabled={blocked}
              onClick={() => void radio.start(RadioModes.measure)}
            >
              Testar carga do ao vivo
            </Button>
            <Button
              type="submit"
              disabled={blocked || !cost?.enabled}
              onClick={() => void radio.start(RadioModes.live)}
            >
              Ver ao vivo
            </Button>
          </div>
        )
      )}
    </>
  );
}
export function RadioLinkCost({ state }: { state: RadioLinkState }) {
  const { editing, link, cost } = state;
  return (
    <>
      {link && !editing && (
        <p>
          {cost
            ? radioCostLabel(cost)
            : 'Ao vivo desligado: custo dos ticks ainda não medido nesta execução.'}
        </p>
      )}
    </>
  );
}
export function RadioLinkBusyStatus({ state }: { state: RadioLinkState }) {
  const { radio } = state;
  return (
    <>
      {radio.busy && (
        <p role="status">
          Consulta em curso; um enlace por vez. A janela do ao vivo termina em
          até 90 s.
        </p>
      )}
    </>
  );
}
