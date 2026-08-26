"use client";

import { useTransition } from "react";
import { setTheme } from "@/app/theme-action";
import type { Theme } from "@/lib/theme";

const OPTIONS: { value: Theme; label: string; glyph: string }[] = [
  { value: "light", label: "Clair", glyph: "☀" },
  { value: "dark", label: "Sombre", glyph: "☾" },
  { value: "system", label: "Système", glyph: "◐" },
];

/**
 * Bascule de thème.
 *
 * Trois états, pas deux : « système » est le défaut, et le retirer obligerait
 * quelqu'un qui bascule son poste le soir à venir changer ici aussi.
 */
export function ThemeSwitch({ current }: { current: Theme }) {
  const [pending, startTransition] = useTransition();

  return (
    <div
      role="group"
      aria-label="Thème"
      className="flex items-center gap-0.5 rounded-md border border-line p-0.5"
    >
      {OPTIONS.map((option) => (
        <button
          key={option.value}
          type="button"
          title={option.label}
          aria-label={option.label}
          aria-pressed={option.value === current}
          disabled={pending}
          onClick={() => startTransition(() => setTheme(option.value))}
          className={`flex h-6 flex-1 items-center justify-center rounded text-xs transition-colors duration-[120ms] ${
            option.value === current
              ? "bg-surface-2 text-ink"
              : "text-ink-3 hover:text-ink"
          }`}
        >
          <span aria-hidden>{option.glyph}</span>
        </button>
      ))}
    </div>
  );
}
