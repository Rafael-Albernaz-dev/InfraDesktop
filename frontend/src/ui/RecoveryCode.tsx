export function RecoveryCode({ value }: { value: string }) {
  return (
    <textarea
      className="codigo codigo-control"
      aria-label="Código de recuperação"
      readOnly
      value={value}
      rows={Math.max(1, Math.ceil(value.length / 20))}
      onFocus={(event) => event.currentTarget.select()}
    />
  );
}
