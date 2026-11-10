import { Reading } from './radio-reading';
import { RadioRoles } from '../equipment/radio-contract';
import type { RadioEquipmentState } from './radio-equipment-state';
import type { RadioReading } from './radio-api';
type Ready = RadioEquipmentState & { reading: RadioReading };
export function RadioReadingDetails({ state }: { state: Ready }) {
  return (
    <details className="det">
      <summary>Detalhes da leitura</summary>
      <Reading
        reading={state.reading}
        standalone
        code={state.radio.view?.ap_code}
        title={
          state.device.role === RadioRoles.ap
            ? 'Clientes do AP'
            : 'Perspectiva da CPE'
        }
      />
    </details>
  );
}
