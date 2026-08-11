import Link from "next/link";
import type { ComponentProps } from "react";

type Variant = "primary" | "secondary" | "ghost";
type Size = "sm" | "md" | "lg";

const base =
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-lg font-medium " +
  "transition-[background-color,border-color,color,transform] duration-[120ms] ease-[var(--ease-out-quint)] " +
  "active:translate-y-px disabled:pointer-events-none disabled:opacity-50";

const variants: Record<Variant, string> = {
  primary:
    "bg-accent text-white hover:bg-accent-hover shadow-[0_1px_0_0_rgba(255,255,255,0.14)_inset,0_8px_24px_-12px_var(--color-accent)]",
  secondary:
    "bg-surface-2 text-ink border border-line hover:border-line-strong hover:bg-surface-3",
  ghost: "text-ink-2 hover:text-ink hover:bg-surface-2",
};

const sizes: Record<Size, string> = {
  sm: "h-8 px-3 text-xs",
  md: "h-9.5 px-4 text-sm",
  lg: "h-11 px-5 text-base",
};

function classes(variant: Variant, size: Size, className?: string) {
  return [base, variants[variant], sizes[size], className].filter(Boolean).join(" ");
}

export function Button({
  variant = "primary",
  size = "md",
  className,
  ...props
}: ComponentProps<"button"> & { variant?: Variant; size?: Size }) {
  return <button className={classes(variant, size, className)} {...props} />;
}

export function ButtonLink({
  variant = "primary",
  size = "md",
  className,
  ...props
}: ComponentProps<typeof Link> & { variant?: Variant; size?: Size }) {
  return <Link className={classes(variant, size, className)} {...props} />;
}
