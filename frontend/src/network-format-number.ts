export const formatter = (digits = 0, minimum = 0) =>
  new Intl.NumberFormat('pt-BR', {
    maximumFractionDigits: digits,
    minimumFractionDigits: minimum,
  });
export const digits = (n: number) => {
  if (n < 10) return 2;
  if (n < 100) return 1;
  return 0;
};
