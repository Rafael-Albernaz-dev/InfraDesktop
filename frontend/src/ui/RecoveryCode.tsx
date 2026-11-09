export function RecoveryCode({
  value,
  label = 'Código de recuperação',
}: {
  value: string;
  label?: string;
}) {
  return (
    <textarea
      className="codigo codigo-control"
      aria-label={label}
      readOnly
      value={value}
      rows={Math.max(1, Math.ceil(value.length / 20))}
      onFocus={(event) => event.currentTarget.select()}
    />
  );
}
