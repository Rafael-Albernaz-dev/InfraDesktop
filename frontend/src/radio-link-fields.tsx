import { Select } from './ui/Select';

import type { RadioLinkState } from './radio-link-state';

export function RadioLinkDeviceSelect({
  state,
  field,
  role,
  label,
}: {
  state: RadioLinkState;
  field: 'cpe_id' | 'ap_id';
  role: string;
  label: string;
}) {
  const { choice, setChoice, devices } = state;
  return (
    <label>
      {label}{' '}
      <Select
        required
        aria-label={label + ' do vínculo'}
        value={choice[field]}
        onChange={(e) => setChoice({ ...choice, [field]: e.target.value })}
      >
        <option value="">Selecione</option>
        {devices
          .filter((d) => d.role === role)
          .map((d) => (
            <option key={d.id} value={d.id}>
              {d.name} · {d.address}
            </option>
          ))}
      </Select>
    </label>
  );
}
export function RadioManualIdentityFields({
  state,
}: {
  state: RadioLinkState;
}) {
  const { choice, setChoice } = state;
  return (
    <>
      {[
        ['mac', 'MAC wireless da CPE'],
        ['interface', 'Interface do AP'],
        ['bssid', 'BSSID do AP'],
      ].map(([key, label]) => (
        <label key={key}>
          {label}
          <input
            required
            aria-label={label}
            maxLength={80}
            value={choice[key as 'mac' | 'interface' | 'bssid']}
            onChange={(e) => setChoice({ ...choice, [key]: e.target.value })}
          />
        </label>
      ))}
    </>
  );
}
