import { formatRate } from '../equipment/network-format';
import type { PPContext } from '../../platform/wails/pppoe-types';

export function sessionPlan(context?: PPContext): string {
  return context?.secret_state === 'active' ||
    context?.secret_state === 'disabled'
    ? context.profile || 'Não informado nesta leitura'
    : 'Não informado nesta leitura';
}

export function planDiffers(context?: PPContext): boolean {
  if (!context) return false;
  const {
    upload_limit: up,
    download_limit: down,
    profile_upload_limit: planUp,
    profile_download_limit: planDown,
  } = context;
  return (
    [up, down, planUp, planDown].every(
      (rate) => rate !== undefined && rate !== '',
    ) &&
    (up !== planUp || down !== planDown)
  );
}

export function appliedSpeed(context?: PPContext): string {
  if (!context?.download_limit || !context.upload_limit)
    return 'Não informado nesta leitura';
  const down = formatRate(context.download_limit),
    up = formatRate(context.upload_limit);
  const [downValue, downUnit] = down.split(' '),
    [upValue, upUnit] = up.split(' ');
  return downUnit && downUnit === upUnit
    ? `${downValue} / ${upValue} ${downUnit}`
    : `${down} / ${up}`;
}
