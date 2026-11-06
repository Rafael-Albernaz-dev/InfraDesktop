import { Button } from './ui/Button';

import type { RadioLinkState } from './radio-link-state';
import { RadioModes } from './radio-contract';
import { radioCostLabel } from './radio-cost-label';

export function RadioLinkRunActions({
  state,
  part = 'all',
}: {
  state: RadioLinkState;
  part?: 'all' | 'update' | 'measurement';
}) {
  const { blocked, radio, editing, link, cost } = state;
  if (radio.busy)
    return part === 'measurement' ? null : (
      <Button
        type="submit"
        className="primary"
        onClick={() => void radio.stop()}
      >
        Parar
      </Button>
    );
  if (!link || editing) return null;
  const update = (
    <Button
      type="submit"
      className={part === 'all' ? 'primary' : ''}
      disabled={blocked}
      onClick={() => void radio.start(RadioModes.once)}
    >
      Atualizar
    </Button>
  );
  if (part === 'update') return update;
  return (
    <div className={part === 'measurement' ? 'actions' : 'radio-actions'}>
      {part === 'all' && update}
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
  );
}
export function RadioLinkCost({
  state,
  session = false,
}: {
  state: RadioLinkState;
  session?: boolean;
}) {
  const { editing, link, cost } = state;
  return (
    <>
      {link && !editing && (
        <p>
          {cost
            ? radioCostLabel(cost)
            : session
              ? 'Ao vivo desligado: custo ainda não medido.'
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
