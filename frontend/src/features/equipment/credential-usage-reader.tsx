import { equipmentAPI } from './equipment-api';

export type CredentialUsage = Record<string, string[]>;

export async function readCredentialUsage(
  profile: string,
  activation: string,
): Promise<CredentialUsage | undefined> {
  const names: CredentialUsage = {};
  const seen = new Set<string>();
  let total: number | undefined;
  for (const inactive of [false, true]) {
    let cursor = 0;
    const cursors = new Set<number>();
    do {
      if (cursors.has(cursor)) return undefined;
      cursors.add(cursor);
      const result = await equipmentAPI.list(
        profile,
        activation,
        '',
        cursor,
        inactive,
      );
      if (
        result.code !== 'OK' ||
        result.profile_id !== profile ||
        result.activation_id !== activation ||
        !result.data
      )
        return undefined;
      const pageTotal = result.data.total_devices;
      if (
        pageTotal === undefined ||
        (total !== undefined && pageTotal !== total)
      )
        return undefined;
      total = pageTotal;
      for (const device of result.data.devices) {
        if (seen.has(device.id)) return undefined;
        seen.add(device.id);
        if (device.credential_id)
          (names[device.credential_id] ||= []).push(device.name);
      }
      cursor = result.data.next_cursor;
    } while (cursor);
  }
  if (seen.size !== total) return undefined;
  for (const list of Object.values(names))
    list.sort((a, b) => a.localeCompare(b, 'pt-BR'));
  return names;
}
