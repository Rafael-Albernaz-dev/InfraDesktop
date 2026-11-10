import { formatVolume } from './network-format';
// Presentation checks give immediate feedback; the daemon remains authoritative.
export const fieldForEquipmentCode: Record<string, string> = {
  DEVICE_NAME_INVALID: 'name',
  DEVICE_ADDRESS_INVALID: 'address',
  DEVICE_PORT_INVALID: 'port',
  CREDENTIAL_ALIAS_INVALID: 'alias',
  CREDENTIAL_USER_INVALID: 'user',
  CREDENTIAL_SECRET_INVALID: 'secret',
  CREDENTIAL_NOT_FOUND: 'credential_id',
};
export const fieldMessages: Record<string, string> = {
  name: 'Informe um nome de até 80 caracteres, sem caracteres de controle.',
  address:
    'Use IPv4 com pontos (10.0.0.1) ou IPv6 (2001:db8::1), sem usuário, URL ou porta.',
  port: 'Informe uma porta SSH inteira de 1 a 65535.',
  alias: 'Informe um alias de até 80 caracteres, sem caracteres de controle.',
  user: 'Informe um usuário de até 80 caracteres, sem caracteres de controle.',
  secret: `Informe a senha, com no máximo ${formatVolume(1024)}.`,
  credential_id: 'Selecione uma credencial desta empresa.',
};
export const validEquipmentText = (s: string) => {
  const value = s.trim();
  return (
    [...value].length > 0 && [...value].length <= 80 && !/\p{Cc}/u.test(value)
  );
};
export const validEquipmentSecret = (s: string) =>
  s.length > 0 && new TextEncoder().encode(s).length <= 1024;
function ipv4(s: string): number[] | undefined {
  const parts = s.split('.');
  if (
    parts.length !== 4 ||
    parts.some((p) => !/^(0|[1-9][0-9]{0,2})$/.test(p) || Number(p) > 255)
  )
    return;
  return parts.map(Number);
}
export function validEquipmentAddress(raw: string): boolean {
  let s = raw.trim();
  if (!s.includes(':')) {
    const v = ipv4(s);
    return Boolean(
      v && v.some((n) => n !== 0) && !(v[0] >= 224 && v[0] <= 239),
    );
  }
  if (s.includes('.')) {
    const pos = s.lastIndexOf(':');
    const tail = ipv4(s.slice(pos + 1));
    if (!tail) return false;
    s =
      s.slice(0, pos + 1) +
      ((tail[0] << 8) | tail[1]).toString(16) +
      ':' +
      ((tail[2] << 8) | tail[3]).toString(16);
  }
  const halves = s.split('::');
  if (halves.length > 2) return false;
  const left = halves[0] ? halves[0].split(':') : [];
  const right = halves.length === 2 && halves[1] ? halves[1].split(':') : [];
  if ([...left, ...right].some((p) => !/^[0-9a-f]{1,4}$/i.test(p)))
    return false;
  const count = left.length + right.length;
  if (halves.length === 1 ? count !== 8 : count >= 8) return false;
  const words = [
    ...left.map((p) => parseInt(p, 16)),
    ...Array<number>(8 - count).fill(0),
    ...right.map((p) => parseInt(p, 16)),
  ];
  return words.some((n) => n !== 0) && words[0] >> 8 !== 0xff;
}
