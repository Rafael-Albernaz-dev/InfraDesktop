import { useEffect, useLayoutEffect, useRef } from 'react';
import type { CSSProperties } from 'react';
import uPlot from 'uplot';
import 'uplot/dist/uPlot.min.css';

export type ChartProps = {
  x: number[];
  series: {
    label: string;
    token: '--chart-down' | '--chart-up';
    values: (number | null)[];
  }[];
  format: (value: number | null) => string;
  formatX: (x: number) => string;
  xSteps: number[];
  markers?: number[];
  live?: boolean;
  label: string;
};

type Current = { current: ChartProps };
type Palette = { token: (name: string) => string; axis: string; grid: string };

function palette(el: HTMLElement): Palette {
  const css = getComputedStyle(el);
  const token = (name: string) => css.getPropertyValue(name).trim();
  return { token, axis: token('--g66'), grid: token('--chart-grade') };
}

function gradient(color: string) {
  return (u: uPlot) => {
    const fill = u.ctx.createLinearGradient(
      0,
      u.bbox.top,
      0,
      u.bbox.top + u.bbox.height,
    );
    fill.addColorStop(0, color + '59');
    fill.addColorStop(1, color + '05');
    return fill;
  };
}

function drawGaps(u: uPlot, color: string) {
  const [x, y] = u.data;
  if (!y) return;
  const { ctx, bbox } = u;
  ctx.save();
  ctx.fillStyle = color;
  ctx.globalAlpha = 0.55;
  for (let i = 0; i < x.length; i++) {
    if (y[i] != null) continue;
    let end = i;
    while (end + 1 < x.length && y[end + 1] == null) end++;
    const a = u.valToPos(x[Math.max(i - 1, 0)], 'x', true);
    const b = u.valToPos(x[Math.min(end + 1, x.length - 1)], 'x', true);
    ctx.fillRect(a, bbox.top, b - a, bbox.height);
    i = end;
  }
  ctx.restore();
}

function drawMarkers(u: uPlot, p: ChartProps, colors: Palette) {
  for (const i of p.markers || []) {
    const y = u.data[1]?.[i];
    if (y == null || u.data[0][i] == null) continue;
    const { ctx } = u;
    ctx.save();
    ctx.fillStyle = colors.token('--chart-alerta');
    ctx.strokeStyle = colors.token('--gff');
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.arc(
      u.valToPos(u.data[0][i], 'x', true),
      u.valToPos(y, 'y', true),
      4 * devicePixelRatio,
      0,
      7,
    );
    ctx.fill();
    ctx.stroke();
    ctx.restore();
  }
}

function tooltip(u: uPlot, tip: HTMLElement, el: HTMLElement, p: ChartProps) {
  const i = u.cursor.idx;
  tip.replaceChildren();
  if (i == null || (u.cursor.left ?? -1) < 0) {
    tip.style.display = 'none';
    return;
  }
  const time = el.ownerDocument.createElement('small');
  time.textContent = p.formatX(u.data[0][i]);
  tip.append(time);
  p.series.forEach((s, j) => {
    const line = el.ownerDocument.createElement('div'),
      dot = el.ownerDocument.createElement('i'),
      value = el.ownerDocument.createElement('b');
    dot.style.background = `var(${s.token})`;
    const y = u.data[j + 1]?.[i];
    value.textContent = y == null ? 'Sem amostra' : p.format(y);
    line.append(dot, el.ownerDocument.createTextNode(s.label), value);
    tip.append(line);
  });
  if (p.markers?.includes(i)) {
    const drop = el.ownerDocument.createElement('small');
    drop.textContent = 'Queda brusca';
    tip.append(drop);
  }
  tip.style.display = 'block';
  const left = u.bbox.left / devicePixelRatio + (u.cursor.left ?? 0),
    width = tip.offsetWidth;
  tip.style.left = `${left + 12 + width > el.clientWidth ? left - width - 12 : left + 12}px`;
  tip.style.top = '10px';
}

function axes(p: Current, colors: Palette): uPlot.Axis[] {
  return [
    {
      stroke: colors.axis,
      font: '11px system-ui, sans-serif',
      size: 26,
      space: 72,
      incrs: p.current.xSteps,
      grid: { show: false },
      ticks: { stroke: colors.grid, size: 4 },
      values: (_u, values) => values.map(p.current.formatX),
    },
    {
      stroke: colors.axis,
      font: '11px system-ui, sans-serif',
      size: 62,
      space: 36,
      grid: { stroke: colors.grid, width: 1 },
      ticks: { show: false },
      values: (_u, values) => values.map(p.current.format),
    },
  ];
}

function options(el: HTMLElement, tip: HTMLElement, p: Current): uPlot.Options {
  const colors = palette(el);
  return {
    width: el.clientWidth,
    height: el.clientHeight,
    legend: { show: false },
    padding: [8, 22, 0, 0],
    series: [
      {},
      ...p.current.series.map((s) => ({
        label: s.label,
        stroke: colors.token(s.token),
        width: 1.75,
        fill: gradient(colors.token(s.token)),
        points: { show: false },
        paths: uPlot.paths.spline!(),
        spanGaps: false,
      })),
    ],
    cursor: {
      y: false,
      drag: { x: false, y: false, setScale: false },
      points: { size: 8, width: 2, stroke: colors.token('--gff') },
    },
    scales: {
      x: { time: false },
      y: {
        range: (_u, _min, max) => [
          0,
          uPlot.rangeNum(0, Math.max(max, 1), 0.1, true)[1],
        ],
      },
    },
    axes: axes(p, colors),
    hooks: {
      drawClear: [(u) => drawGaps(u, palette(el).grid)],
      draw: [(u) => drawMarkers(u, p.current, palette(el))],
      setCursor: [(u) => tooltip(u, tip, el, p.current)],
    },
  };
}

function redrawTheme(plot: uPlot, el: HTMLElement, props: ChartProps) {
  const colors = palette(el);
  props.series.forEach((s, i) => {
    plot.series[i + 1].stroke = () => colors.token(s.token);
    plot.series[i + 1].fill = gradient(colors.token(s.token));
  });
  plot.axes.forEach((axis, i) => {
    axis.stroke = () => colors.axis;
    axis.ticks!.stroke = () => colors.grid;
    if (i === 1) axis.grid!.stroke = () => colors.grid;
  });
  plot.redraw(true, true);
}

function last(values: (number | null)[]) {
  for (let i = values.length - 1; i >= 0; i--)
    if (values[i] != null) return values[i];
  return null;
}

export function Chart(props: ChartProps) {
  const host = useRef<HTMLDivElement>(null),
    plot = useRef<uPlot | null>(null),
    current = useRef(props);
  useLayoutEffect(() => {
    current.current = props;
  });
  useEffect(() => {
    const el = host.current!;
    const tip = el.ownerDocument.createElement('div');
    tip.className = 'ch-tip';
    el.append(tip);
    const instance = new uPlot(
      options(el, tip, current),
      [current.current.x, ...current.current.series.map((s) => s.values)],
      el,
    );
    plot.current = instance;
    const resize = new ResizeObserver(() =>
      instance.setSize({ width: el.clientWidth, height: el.clientHeight }),
    );
    resize.observe(el);
    const theme = new MutationObserver(() =>
      redrawTheme(instance, el, current.current),
    );
    for (
      let node: HTMLElement | null = el.parentElement;
      node;
      node = node.parentElement
    )
      theme.observe(node, {
        attributes: true,
        attributeFilter: ['data-theme', 'class'],
      });
    return () => {
      resize.disconnect();
      theme.disconnect();
      instance.destroy();
      tip.remove();
      plot.current = null;
    };
  }, []);
  useEffect(() => {
    plot.current?.setData([props.x, ...props.series.map((s) => s.values)]);
  }, [props.x, props.series]);
  return (
    <>
      <div className="ch-top">
        {props.series.map((s) => (
          <span
            key={s.label}
            className="ch-stat"
            style={{ '--cor': `var(${s.token})` } as CSSProperties}
          >
            <i />
            {s.label} <b>{props.format(last(s.values))}</b>
          </span>
        ))}
        {props.live !== undefined && (
          <span className={`ch-vivo ${props.live ? 'on' : ''}`}>
            <i />
            {props.live ? 'Ao vivo' : 'Parado'}
          </span>
        )}
      </div>
      <div ref={host} className="ch-plot" role="img" aria-label={props.label} />
    </>
  );
}
