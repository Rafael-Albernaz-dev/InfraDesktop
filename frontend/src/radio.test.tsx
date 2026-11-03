import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { RadioLinkPanel } from './radio';
import { radioAPI as api } from './radio-api';
import type { RadioResult } from './radio-api';
import { device, response, reading, link } from './radio-test-fixtures';

vi.mock('./radio-api', () => ({
  radioAPI: {
    current: vi.fn(),
    start: vi.fn(),
    status: vi.fn(),
    cancel: vi.fn(),
    cancelOperation: vi.fn(),
    confirmFamily: vi.fn(),
    link: vi.fn(),
    confirmLink: vi.fn(),
  },
}));

vi.mock('./equipment-api', () => ({
  equipmentMessage: (c: string) => c,
  equipmentAPI: {
    list: vi.fn(),
    get: vi.fn(),
    credentials: vi.fn(),
    save: vi.fn(),
  },
}));

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.current).mockResolvedValue(response());
  vi.mocked(api.cancel).mockResolvedValue(response());
  vi.mocked(api.cancelOperation).mockResolvedValue(response());
  vi.mocked(api.link).mockResolvedValue(response({ link }));
});

it('hiding stops the live and returning never renews it; a late start is cancelled on unmount', async () => {
  vi.mocked(api.link).mockResolvedValue(
    response({
      link,
      cost: {
        samples: Array(10).fill(1),
        enabled: true,
        reason: 'Custo medido.',
        p95_seconds: 1,
        interval_seconds: 5,
      },
    }),
  );
  let finish!: (r: RadioResult) => void;
  vi.mocked(api.start).mockReturnValue(
    new Promise((resolve) => {
      finish = resolve;
    }),
  );
  const view = render(
    <RadioLinkPanel
      profile="company"
      activation="active"
      device={{ ...device, id: 'concentrator' }}
      observation="observation"
      row="row"
      pppoeUptime="1 h"
      blocked={false}
    />,
  );
  fireEvent.click(await screen.findByRole('button', { name: 'Live curto' }));
  expect(vi.mocked(api.start).mock.calls[0][2].mode).toBe('live');
  view.unmount();
  await act(async () =>
    finish(response({ job: { id: 'late-job', state: 'RUNNING', code: 'OK' } })),
  );
  await waitFor(() => expect(api.cancelOperation).toHaveBeenCalled());
  expect(api.status).not.toHaveBeenCalled();
  expect(api.start).toHaveBeenCalledTimes(1);
});

it('visibility loss cancels an active job exactly once and never issues a replacement read', async () => {
  prepareVisibilityCancellation();
  render(
    <RadioLinkPanel
      profile="company"
      activation="active"
      device={{ ...device, id: 'concentrator' }}
      observation="observation"
      row="row"
      pppoeUptime="1 h"
      blocked={false}
    />,
  );
  fireEvent.click(await screen.findByRole('button', { name: 'Live curto' }));
  await waitFor(() => expect(api.status).toHaveBeenCalled());
  Object.defineProperty(
    screen.getByRole('region', { name: 'Enlace rádio' }).ownerDocument,
    'visibilityState',
    {
      configurable: true,
      value: 'hidden',
    },
  );
  fireEvent(
    screen.getByRole('region', { name: 'Enlace rádio' }).ownerDocument,
    new Event('visibilitychange'),
  );
  await waitFor(() =>
    expect(api.cancel).toHaveBeenCalledWith('company', 'active', 'live-job'),
  );
  Object.defineProperty(
    screen.getByRole('region', { name: 'Enlace rádio' }).ownerDocument,
    'visibilityState',
    {
      configurable: true,
      value: 'visible',
    },
  );
  fireEvent(
    screen.getByRole('region', { name: 'Enlace rádio' }).ownerDocument,
    new Event('visibilitychange'),
  );
  expect(api.start).toHaveBeenCalledTimes(1);
  expect(api.cancel).toHaveBeenCalledTimes(1);
});

it('R12: Parar followed by a delayed start cannot make the next stop target the old job', async () => {
  const { first, second } = prepareDelayedStopReplies();
  render(
    <RadioLinkPanel
      profile="company"
      activation="active"
      device={{ ...device, id: 'concentrator' }}
      observation="observation"
      row="row"
      pppoeUptime="1 h"
      blocked={false}
    />,
  );
  fireEvent.click(
    await screen.findByRole('button', { name: 'Consultar enlace' }),
  );
  fireEvent.click(screen.getByRole('button', { name: 'Parar' }));
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Consultar enlace' }),
    ).toBeEnabled(),
  );
  await act(async () =>
    first(
      response({
        job: { id: 'stopped-late-job', state: 'RUNNING', code: 'OK' },
      }),
    ),
  );
  fireEvent.click(screen.getByRole('button', { name: 'Consultar enlace' }));
  fireEvent.click(screen.getByRole('button', { name: 'Parar' }));
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Consultar enlace' }),
    ).toBeEnabled(),
  );
  expect(api.cancel).not.toHaveBeenCalled();
  expect(api.cancelOperation).toHaveBeenCalledWith(
    'company',
    'active',
    'concentrator',
    vi.mocked(api.start).mock.calls[1][2].operation_id,
  );
  await act(async () =>
    second(
      response({
        job: { id: 'second-stopped-job', state: 'RUNNING', code: 'OK' },
      }),
    ),
  );
  expect(api.status).not.toHaveBeenCalled();
  expect(api.start).toHaveBeenCalledTimes(2);
});

function prepareVisibilityCancellation() {
  vi.mocked(api.link).mockResolvedValue(
    response({
      link,
      cost: {
        samples: Array(10).fill(1),
        enabled: true,
        reason: 'Medido',
        p95_seconds: 1,
        interval_seconds: 5,
      },
    }),
  );
  vi.mocked(api.start).mockResolvedValue(
    response({ job: { id: 'live-job', state: 'RUNNING', code: 'OK' } }),
  );
  vi.mocked(api.status).mockResolvedValue(
    response({
      link,
      reading,
      job: { id: 'live-job', state: 'RUNNING', code: 'OK' },
    }),
  );
}

function prepareDelayedStopReplies() {
  let first!: (r: RadioResult) => void, second!: (r: RadioResult) => void;
  vi.mocked(api.start)
    .mockReturnValueOnce(
      new Promise((resolve) => {
        first = resolve;
      }),
    )
    .mockReturnValueOnce(
      new Promise((resolve) => {
        second = resolve;
      }),
    );
  return { first, second };
}
