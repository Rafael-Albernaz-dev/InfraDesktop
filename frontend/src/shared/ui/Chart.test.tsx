import { render } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { Chart } from './Chart';
import type { ChartProps } from './Chart';

const calls = vi.hoisted(() => ({
  create: vi.fn(),
  setData: vi.fn(),
  destroy: vi.fn(),
}));
vi.mock('uplot', () => {
  class Plot {
    static paths = { spline: () => () => null };
    static rangeNum = () => [0, 1];
    setData = calls.setData;
    destroy = calls.destroy;
    constructor(...args: unknown[]) {
      calls.create(...args);
    }
    setSize() {}
  }
  return { default: Plot };
});

it('preserva null nas lacunas, atualiza a mesma instância e a destrói ao desmontar', () => {
  const props: ChartProps = {
    x: [1, 2, 3],
    series: [
      { label: 'Download', token: '--chart-down', values: [3, null, 5] },
    ],
    format: String,
    formatX: String,
    xSteps: [1],
    label: 'Tráfego',
  };
  const view = render(<Chart {...props} />);
  expect(calls.create.mock.calls[0][1]).toEqual([
    [1, 2, 3],
    [3, null, 5],
  ]);
  view.rerender(
    <Chart
      {...props}
      x={[1, 2, 3, 4]}
      series={[{ ...props.series[0], values: [3, null, 5, 7] }]}
    />,
  );
  expect(calls.create).toHaveBeenCalledOnce();
  expect(calls.setData).toHaveBeenLastCalledWith([
    [1, 2, 3, 4],
    [3, null, 5, 7],
  ]);
  view.unmount();
  expect(calls.destroy).toHaveBeenCalledOnce();
});
