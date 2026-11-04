export function ProductBrand() {
  return (
    <>
      <svg className="marca" aria-hidden="true" viewBox="0 0 36 36">
        <rect width="36" height="36" rx="5" fill="var(--brand-background)" />
        <g
          fill="none"
          stroke="var(--brand-foreground)"
          strokeWidth="1.3"
          strokeLinecap="butt"
          strokeLinejoin="miter"
        >
          <rect x="14.75" y="7.4375" width="6.5" height="5.6875" rx="0.8125" />
          <rect x="7.4375" y="22.875" width="6.5" height="5.6875" rx="0.8125" />
          <rect
            x="22.0625"
            y="22.875"
            width="6.5"
            height="5.6875"
            rx="0.8125"
          />
          <path d="M18 13.125 L18 18" />
          <path d="M10.6875 22.875 L10.6875 18 L25.3125 18 L25.3125 22.875" />
        </g>
      </svg>
      <span>InfraDesktop</span>
    </>
  );
}
