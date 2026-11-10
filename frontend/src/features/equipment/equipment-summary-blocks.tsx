import { Fragment } from 'react';
import type { ReactNode } from 'react';
import type { Device } from './equipment-api';
import {
  formatMetric,
  formatRouterDuration,
  exactVolume,
  formatVolumePair,
} from './network-format';
import { stamp } from './equipment-reading-format';
import { DataBlock } from '../../shared/ui/DataBlock';
import { readingUpdateState } from './reading-update-state';
import type { ReadingUpdateStatus } from './reading-update-state';
export function Summary({
  device,
  statistics,
  status,
}: {
  device: Device;
  statistics?: ReactNode;
  status?: ReadingUpdateStatus;
}) {
  const r = device.result!;
  const pairs = [
    ['Identidade', r.identity],
    ['Modelo', r.board],
    ['Versão', r.version],
    ['Uptime', formatRouterDuration(r.uptime)],
    [
      'Origem da leitura',
      r.quality === 'complete'
        ? 'Resumo completo'
        : 'Leitura com parte limitada',
    ],
  ];
  return (
    <div className="grid">
      <DataBlock
        title="Identidade e sistema"
        source="SSH"
        age={stamp(device)}
        data={readingUpdateState(
          <>
            <dl className="kv">
              {pairs.map(([key, value]) => (
                <Fragment key={key}>
                  <dt>{key}</dt>
                  <dd>{value || 'Não informado nesta leitura'}</dd>
                </Fragment>
              ))}
            </dl>
          </>,
          status,
        )}
      />
      <DataBlock
        title="Recursos"
        source="SSH"
        age={stamp(device)}
        data={readingUpdateState(
          <dl className="kv">
            <dt>CPU</dt>
            <dd>{formatMetric(r.cpu_percent, '%')}</dd>
            <dt>Memória livre / total</dt>
            <dd
              title={`${exactVolume(r.free_memory_bytes)} / ${exactVolume(r.total_memory_bytes)}`}
            >
              {formatVolumePair(r.free_memory_bytes, r.total_memory_bytes)}
            </dd>
          </dl>,
          status && { ...status, error: undefined },
        )}
      />
      {statistics}
    </div>
  );
}
