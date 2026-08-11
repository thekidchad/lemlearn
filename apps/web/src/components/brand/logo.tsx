import type { SVGProps } from "react";

/**
 * Marque lemlearn : un triangle de lecture traversé par une coche qui s'échappe
 * vers le haut. Les deux moitiés du produit dans une seule forme — la vidéo
 * suivie, et la preuve qui en découle.
 *
 * La coche est tracée deux fois : d'abord en couleur de fond pour se détacher
 * du triangle, puis en couleur d'accent. Sur fond sombre, un simple
 * chevauchement rendrait la forme illisible.
 */
export function LogoMark({ className, ...props }: SVGProps<SVGSVGElement>) {
  return (
    <svg viewBox="0 0 32 32" fill="none" aria-hidden="true" className={className} {...props}>
      <path
        d="M8.6 6.2 20.4 14.4a1.9 1.9 0 0 1 0 3.2L8.6 25.8A1.4 1.4 0 0 1 6.4 24.6V7.4a1.4 1.4 0 0 1 2.2-1.2Z"
        fill="currentColor"
      />
      <path
        d="m11.6 16.9 4.6 4.9L27.2 6.6"
        stroke="var(--logo-knockout, #0b0c0f)"
        strokeWidth="6.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="m11.6 16.9 4.6 4.9L27.2 6.6"
        stroke="currentColor"
        strokeWidth="3.2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

/** Variante encadrée : icône d'application, favicon, en-tête de PDF. */
export function LogoSquare({ className, ...props }: SVGProps<SVGSVGElement>) {
  return (
    <svg viewBox="0 0 40 40" fill="none" aria-hidden="true" className={className} {...props}>
      <rect width="40" height="40" rx="11" fill="#0b0c0f" />
      <rect
        x="0.5"
        y="0.5"
        width="39"
        height="39"
        rx="10.5"
        stroke="#2b2e38"
        strokeWidth="1"
      />
      <g transform="translate(4 4)" fill="#7c5cff" stroke="#7c5cff">
        <path
          d="M8.6 6.2 20.4 14.4a1.9 1.9 0 0 1 0 3.2L8.6 25.8A1.4 1.4 0 0 1 6.4 24.6V7.4a1.4 1.4 0 0 1 2.2-1.2Z"
          stroke="none"
        />
        <path
          d="m11.6 16.9 4.6 4.9L27.2 6.6"
          fill="none"
          stroke="#0b0c0f"
          strokeWidth="6.5"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <path
          d="m11.6 16.9 4.6 4.9L27.2 6.6"
          fill="none"
          strokeWidth="3.2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </g>
    </svg>
  );
}

export function Logo({ className }: { className?: string }) {
  return (
    <span className={`inline-flex items-center gap-2.5 ${className ?? ""}`}>
      <LogoMark className="size-7 text-accent" />
      <span className="text-[0.9375rem] font-semibold tracking-[-0.045em] text-ink">
        lemlearn
      </span>
    </span>
  );
}
