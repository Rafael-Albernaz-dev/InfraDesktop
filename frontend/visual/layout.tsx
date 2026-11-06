// Isolated synthetic fixture: imports the product views, never the frozen renderer.
import { createRoot } from 'react-dom/client';
import { useEffect, useState } from 'react';
import { AccessScreen } from '../src/access-screen';
import { Desktop } from '../src/desktop';
import { Equipment } from '../src/equipment';
import type { Page } from '../src/desktop';
import { DataBlock } from '../src/ui/DataBlock';
import type { DataState } from '../src/ui/DataBlock';
import { Chart } from '../src/ui/Chart';
import '../src/ui/tokens.css';
import '../src/ui/kit.css';
import { installFixture } from './layout-fixture';
const query = new URLSearchParams(location.search),
  scene = query.get('scene') || 'login';
installFixture();
const noop = () => {};
function Vitrine() {
  const states: [string, DataState][] = [
    ['Sem leitura', { state: 'unread', action: noop }],
    ['Carregando', { state: 'loading' }],
    [
      'Atualizando',
      {
        state: 'refreshing',
        content: (
          <dl className="kv">
            <dt>CPU</dt>
            <dd>8%</dd>
            <dt>Memória</dt>
            <dd>41%</dd>
          </dl>
        ),
      },
    ],
    [
      'Pronto',
      {
        state: 'ready',
        content: (
          <dl className="kv">
            <dt>CPU</dt>
            <dd>8%</dd>
            <dt>Temperatura</dt>
            <dd>Não informado nesta leitura</dd>
          </dl>
        ),
      },
    ],
    ['Vazio', { state: 'empty', message: 'Nenhum reinício encontrado' }],
    [
      'Falhou',
      {
        state: 'failed',
        reason: 'Credencial recusada pelo equipamento',
        technical: 'SSH_TIMEOUT · tentativa há 2 min',
        retry: noop,
      },
    ],
    ['Sem suporte', { state: 'not-supported', resource: 'temperatura' }],
    ['Não feito', { state: 'unbuilt', slice: 'L-ROT' }],
  ];
  const epoch = 1793800000,
    wave = (i: number, base: number, amp: number) =>
      +(
        base +
        amp *
          (0.55 * Math.sin(i / 2.3) +
            0.3 * Math.sin(i / 1.1 + 1) +
            0.15 * Math.sin(i * 1.7))
      ).toFixed(2);
  const x = Array.from({ length: 30 }, (_, i) => epoch + i * 2),
    values = (base: number, amp: number, shift: number) =>
      x.map((_, i) =>
        i === 16 || i === 17 ? null : wave(i + shift, base, amp),
      );
  const sessions = [
    72,
    74,
    73,
    76,
    79,
    81,
    84,
    83,
    86,
    88,
    null,
    null,
    89,
    91,
    90,
    70,
    78,
    85,
    90,
    92,
    94,
    95,
    97,
    99,
  ];
  const format = (v: number | null) =>
    v == null
      ? '—'
      : v >= 1000
        ? `${(v / 1000).toLocaleString('pt-BR', { maximumFractionDigits: 2 })} Gbps`
        : v >= 1
          ? `${v.toLocaleString('pt-BR', { maximumFractionDigits: 1 })} Mbps`
          : v === 0
            ? '0'
            : `${Math.round(v * 1000)} Kbps`;
  return (
    <>
      <h1>Vitrine — bloco de dado (8 estados)</h1>
      <div className="grid">
        {states.map(([label, data]) => (
          <div key={label}>
            <div className="state">Estado: {label}</div>
            <DataBlock
              title="Título do bloco"
              source="SSH"
              age="há 12 s"
              data={data}
            />
          </div>
        ))}
      </div>
      <h2>Gráfico</h2>
      <div className="pair">
        <DataBlock
          title="Tráfego"
          source="SSH"
          age="há 2 s"
          data={{
            state: 'ready',
            content: (
              <Chart
                label="Tráfego"
                x={x}
                series={[
                  {
                    label: 'Download',
                    token: '--chart-down',
                    values: values(11, 6, 0),
                  },
                  {
                    label: 'Upload',
                    token: '--chart-up',
                    values: values(1.1, 0.45, 4),
                  },
                ]}
                format={format}
                formatX={(t) => new Date(t * 1000).toISOString().slice(11, 19)}
                xSteps={[2, 5, 10, 20, 30, 60, 120, 300]}
                live={false}
              />
            ),
          }}
        />
        <DataBlock
          title="Curva de sessões"
          source="local"
          age="há 6 min"
          data={{
            state: 'ready',
            content: (
              <Chart
                label="Curva de sessões"
                x={sessions.map((_, i) => epoch - 86400 + i * 3600)}
                series={[
                  {
                    label: 'Sessões online',
                    token: '--chart-down',
                    values: sessions,
                  },
                ]}
                format={(v) => (v == null ? '—' : String(Math.round(v)))}
                formatX={(t) => new Date(t * 1000).toISOString().slice(11, 16)}
                xSteps={[3600, 10800, 21600, 43200, 86400]}
                markers={[15]}
              />
            ),
          }}
        />
      </div>
    </>
  );
}
function Scene() {
  const [ready, setReady] = useState(false);
  useEffect(() => {
    requestAnimationFrame(() => requestAnimationFrame(() => setReady(true)));
  }, []);
  const [theme, setTheme] = useState(query.get('theme') || 'light');
  const mode =
    scene === 'criar'
      ? 'register'
      : scene === 'recuperar'
        ? 'recover'
        : 'login';
  const [page, setPage] = useState<Page>(
    scene === 'vitrine'
      ? 'vitrine'
      : scene === 'empresas'
        ? 'companies'
        : scene === 'config'
          ? 'settings'
          : scene === 'visao'
            ? 'overview'
            : scene === 'credenciais'
              ? 'credentials'
              : 'equipment',
  );
  const base = scene.startsWith('ficha-') ? scene.slice(6) : '';
  const equipmentId =
    base.startsWith('ccr-centro') ||
    scene === 'curva-sessoes' ||
    scene.startsWith('sessao-')
      ? 'CCR-CENTRO'
      : base.startsWith('ap-norte')
        ? 'AP-NORTE'
        : base.startsWith('ap-sul')
          ? 'AP-SUL'
          : base.startsWith('cpe-0001')
            ? 'CPE-0001'
            : base.startsWith('ptp-0001')
              ? 'PTP-0001'
              : base.startsWith('eqp-0001')
                ? 'EQP-0001'
                : '';
  return (
    <div data-layout-ready={ready || undefined}>
      {['login', 'criar', 'recuperar', 'codigo'].includes(scene) ? (
        <AccessScreen
          theme={theme}
          setTheme={setTheme}
          setup
          connected
          busy={false}
          error=""
          recovery={scene === 'codigo' ? 'XXXX-XXXX-XXXX' : ''}
          initialMode={mode}
          acknowledge={noop}
          submit={noop}
        />
      ) : (
        <Desktop
          page={page}
          navigate={setPage}
          brand={{ name: 'Empresa-0001', accent: 'teal', logo: '' }}
          account="Administrador"
          theme={theme}
          setTheme={setTheme}
          connected
          busy={false}
          companies={[
            { id: 'visual', name: 'Empresa-0001', count: 5 },
            { id: 'other', name: 'Empresa-0002', count: 0 },
          ]}
          activeProfile="visual"
          create={noop}
          enter={noop}
          logout={noop}
          unavailable={noop}
          security={noop}
          inventory={
            scene === 'vitrine' ? (
              <Vitrine />
            ) : (
              <Equipment
                profile="visual"
                activation="fixture"
                connected
                credentialsPage={page === 'credentials'}
                addPage={page === 'adicionar'}
                onAdd={() => setPage('adicionar')}
                onAddBack={() => setPage('equipment')}
                targetID={equipmentId}
              />
            )
          }
        />
      )}
    </div>
  );
}
createRoot(document.getElementById('root')!).render(<Scene />);
