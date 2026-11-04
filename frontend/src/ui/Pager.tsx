import { Button } from './Button';

export function Pager({
  page,
  total,
  first,
  next,
  hasNext = total !== undefined && page < total,
}: {
  page: number;
  total?: number;
  first: () => void;
  next: () => void;
  hasNext?: boolean;
}) {
  return (
    <div className="pager">
      <Button disabled={page <= 1} onClick={first}>
        Início
      </Button>
      <span>
        Página {page}
        {total ? ` de ${total}` : ''}
      </span>
      <Button disabled={!hasNext} onClick={next}>
        Próxima
      </Button>
    </div>
  );
}
