import type { ComponentProps } from 'react';

export function Checkbox(props: Omit<ComponentProps<'input'>, 'type'>) {
  return (
    <input
      {...props}
      type="checkbox"
      onClick={(event) => {
        event.stopPropagation();
        props.onClick?.(event);
      }}
      onKeyDown={(event) => {
        event.stopPropagation();
        props.onKeyDown?.(event);
      }}
    />
  );
}
