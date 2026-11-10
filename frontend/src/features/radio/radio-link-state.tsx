import type { Dispatch, RefObject, SetStateAction } from 'react';
import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { Device } from '../equipment/equipment-api';
import { equipmentAPI } from '../equipment/equipment-api';
import { useDisplayNow } from '../equipment/equipment-snapshot';
import type { RadioLinkChoice, RadioResult } from '../equipment/radio-api';
import { radioAPI as api } from '../equipment/radio-api';
import { RadioModes, RadioRoles } from '../equipment/radio-contract';
import type { RadioLinkPanelProps } from './radio-link-props';
import { radioMessage } from './radio-messages';
import { useRadio } from './radio-observation';
import { useLatestRef } from '../../shared/hooks/use-latest-ref';
export type { RadioLinkPanelProps } from './radio-link-props';

type LinkContext = Pick<
  RadioLinkPanelProps,
  'profile' | 'activation' | 'device' | 'observation' | 'row'
>;
type RadioController = Omit<ReturnType<typeof useRadio>, 'root'>;
type ContextLife = RefObject<object>;

function useRadioLinkLoad(
  context: LinkContext,
  blocked: boolean,
  radio: RadioController,
) {
  const { profile, activation, device, observation, row } = context;
  const [loading, setLoading] = useState(true),
    [reload, setReload] = useState(0);
  const latestRef = useLatestRef({ radio, blocked });
  useEffect(() => {
    const { radio } = latestRef.current;
    let alive = true;
    void (async () => {
      try {
        const result = await api.link(
          profile,
          activation,
          device.id,
          observation,
          row,
        );
        if (!alive) return;
        if (result.code !== 'OK') {
          radio.setError(radioMessage(result.code));
          return;
        }
        radio.setView(result.data);
        if (
          !latestRef.current.blocked &&
          !result.data?.link &&
          !result.data?.index &&
          result.data?.match_reason
        )
          await latestRef.current.radio.start(RadioModes.index);
      } catch {
        if (alive) radio.setError('Não foi possível abrir o vínculo salvo.');
      } finally {
        if (alive) setLoading(false);
      }
    })();
    return () => {
      alive = false;
    };
    // Same scalar context owns this stored-link read and explicit reload.
  }, [profile, activation, device.id, observation, row, reload, latestRef]);
  const changed = () => {
    radio.setError('');
    setReload((value) => value + 1);
  };
  const reopen = () => {
    radio.setError('');
    setLoading(true);
    setReload((value) => value + 1);
  };
  return { loading, changed, reopen };
}

function useRadioLinkEditor() {
  const [devices, setDevices] = useState<Device[]>([]);
  const [choice, setChoice] = useState<RadioLinkChoice>({
    ap_id: '',
    cpe_id: '',
    mac: '',
    interface: '',
    bssid: '',
    evidence: 'Confirmação manual do técnico',
    origin: 'manual',
  });
  const [editing, setEditing] = useState(false);
  return { devices, setDevices, choice, setChoice, editing, setEditing };
}
type LinkEditor = ReturnType<typeof useRadioLinkEditor>;

function createRadioLinkEdit(
  context: LinkContext,
  radio: RadioController,
  editor: LinkEditor,
) {
  return async function edit() {
    editor.setEditing(true);
    try {
      const all: Device[] = [];
      let cursor = 0;
      for (let page = 0; page < 16; page++) {
        const result = await equipmentAPI.list(
          context.profile,
          context.activation,
          '',
          cursor,
          false,
        );
        if (result.code !== 'OK') throw new Error('inventory');
        all.push(...(result.data?.devices || []));
        if (!result.data?.next_cursor) break;
        cursor = result.data.next_cursor;
      }
      editor.setDevices(
        all.filter(
          (device) =>
            device.role === RadioRoles.ap || device.role === RadioRoles.cpe,
        ),
      );
    } catch {
      radio.setError('Inventário indisponível para confirmar o vínculo.');
    }
  };
}

function useRadioLinkContextLife(context: LinkContext) {
  const { profile, activation, device, observation, row } = context;
  const life = useRef({});
  useLayoutEffect(() => {
    life.current = {};
    return () => {
      life.current = {};
    };
  }, [profile, activation, device.id, observation, row]);
  return life;
}

function createRadioLinkIndexPage(
  context: LinkContext,
  radio: RadioController,
  life: ContextLife,
) {
  return async function indexPage(offset: number) {
    const generation = life.current,
      owner = radio.owner();
    try {
      const result = await api.indexPage(
        context.profile,
        context.activation,
        context.device.id,
        context.observation,
        context.row,
        offset,
      );
      if (generation !== life.current || owner !== radio.owner()) return;
      if (
        result.code === 'OK' &&
        result.profile_id === context.profile &&
        result.activation_id === context.activation
      )
        radio.setView(result.data);
      else radio.setError(radioMessage(result.code));
    } catch {
      if (generation === life.current && owner === radio.owner())
        radio.setError('Não foi possível abrir esta página do índice.');
    }
  };
}

function mergeConfirmedLinkView(
  confirmed: RadioResult['data'],
  refreshed?: RadioResult,
) {
  if (refreshed?.code !== 'OK' || !refreshed.data) return confirmed;
  const hasStoredLink = Boolean(refreshed.data.link);
  return {
    ...confirmed,
    ...refreshed.data,
    link: refreshed.data.link || confirmed?.link,
    reading: refreshed.data.reading || confirmed?.reading,
    suggestions: hasStoredLink ? refreshed.data.suggestions : [],
    proposal: hasStoredLink ? refreshed.data.proposal : undefined,
  };
}

function useRadioLinkConfirmation(
  context: LinkContext,
  radio: RadioController,
  life: ContextLife,
  setEditing: Dispatch<SetStateAction<boolean>>,
) {
  const [saving, setSaving] = useState(false);
  async function confirm(selected: RadioLinkChoice) {
    const generation = life.current;
    if (saving) return;
    setSaving(true);
    radio.setError('');
    try {
      const result = await api.confirmLink(
        context.profile,
        context.activation,
        context.device.id,
        context.observation,
        context.row,
        crypto.randomUUID(),
        selected,
      );
      if (generation !== life.current) return;
      if (result.code !== 'OK') {
        radio.setError(radioMessage(result.code));
        return;
      }
      const refreshed = await api
        .link(
          context.profile,
          context.activation,
          context.device.id,
          context.observation,
          context.row,
        )
        .catch(() => undefined);
      if (generation !== life.current) return;
      radio.setView(mergeConfirmedLinkView(result.data, refreshed));
      setEditing(false);
    } catch {
      if (generation !== life.current) return;
      radio.setError(
        'Confirmação incerta. Reabra o detalhe para conferir o vínculo salvo.',
      );
    } finally {
      if (generation === life.current) setSaving(false);
    }
  }
  return { saving, confirm };
}

export function useRadioLinkState(props: RadioLinkPanelProps) {
  const { root: rootRef, ...radio } = useRadio(
    props.profile,
    props.activation,
    props.device,
    props.observation,
    props.row,
  );
  const loaded = useRadioLinkLoad(props, props.blocked, radio);
  const editor = useRadioLinkEditor();
  const life = useRadioLinkContextLife(props);
  const confirmation = useRadioLinkConfirmation(
    props,
    radio,
    life,
    editor.setEditing,
  );
  const now = useDisplayNow();
  return {
    ...props,
    rootRef,
    radio,
    ...loaded,
    ...editor,
    ...confirmation,
    now,
    edit: createRadioLinkEdit(props, radio, editor),
    indexPage: createRadioLinkIndexPage(props, radio, life),
    link: radio.view?.link,
    cost: radio.view?.cost,
    index: radio.view?.index,
    proposal: radio.view?.proposal,
  };
}
export type RadioLinkState = Omit<
  ReturnType<typeof useRadioLinkState>,
  'rootRef'
>;
