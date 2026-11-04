import { useRef } from 'react';
import { Button } from './Button';

export function Tabs<T extends string>({
  items,
  value,
  onChange,
  label,
  sub = false,
}: {
  items: readonly (readonly [T, string])[];
  value: T;
  onChange: (value: T) => void;
  label: string;
  sub?: boolean;
}) {
  const ref = useRef<HTMLDivElement>(null);
  return (
    <div
      ref={ref}
      role="tablist"
      aria-label={label}
      className={`tabs ${sub ? 'subtabs' : ''}`}
    >
      {items.map(([id, title], i) => (
        <Button
          key={id}
          role="tab"
          aria-selected={value === id}
          tabIndex={value === id ? 0 : -1}
          className={value === id ? 'on' : ''}
          onClick={() => onChange(id)}
          onKeyDown={(event) => {
            const next =
              event.key === 'ArrowRight'
                ? (i + 1) % items.length
                : event.key === 'ArrowLeft'
                  ? (i + items.length - 1) % items.length
                  : event.key === 'Home'
                    ? 0
                    : event.key === 'End'
                      ? items.length - 1
                      : -1;
            if (next < 0) return;
            event.preventDefault();
            onChange(items[next][0]);
            const tabs =
              ref.current?.querySelectorAll<HTMLButtonElement>('[role=tab]');
            tabs?.item(next).focus();
          }}
        >
          {title}
        </Button>
      ))}
    </div>
  );
}
