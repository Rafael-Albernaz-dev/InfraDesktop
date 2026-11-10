import { scrubSecrets } from './network-text';

export function TechnicalDetail(props: {
  detail?: string | null;
  label?: string;
}) {
  const clean = scrubSecrets((props.detail ?? '').trim());
  if (!clean) return null;
  return (
    <details className="technical-detail">
      <summary>{props.label || 'Detalhe técnico'}</summary>
      <code>{clean}</code>
    </details>
  );
}
