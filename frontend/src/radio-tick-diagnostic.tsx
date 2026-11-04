import { formatTickReason, TechnicalDetail } from './network-format';
export function RadioTickDiagnostic({
  failure,
}: {
  failure?: { reason: string };
}) {
  if (!failure) return null;
  const reason = formatTickReason(failure.reason);
  return (
    <>
      <p role="status">{reason.summary}</p>
      <TechnicalDetail detail={reason.technical} />
    </>
  );
}
