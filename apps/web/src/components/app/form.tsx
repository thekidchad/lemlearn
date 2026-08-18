"use client";

import { useActionState, useState } from "react";
import type { FormState } from "@/app/actions/crm";

/**
 * Panneau de création, replié par défaut.
 *
 * Un formulaire toujours déployé pousse l'inventaire — la vraie matière de
 * l'écran — sous la ligne de flottaison. Il s'ouvre à la demande et se referme
 * après un enregistrement réussi.
 */
export function CreatePanel({
  label,
  title,
  action,
  children,
  submitLabel = "Enregistrer",
}: {
  label: string;
  title: string;
  action: (state: FormState, form: FormData) => Promise<FormState>;
  children: React.ReactNode;
  submitLabel?: string;
}) {
  const [open, setOpen] = useState(false);
  const [state, formAction, pending] = useActionState(
    async (previous: FormState, form: FormData) => {
      const result = await action(previous, form);
      if (result.ok) setOpen(false);
      return result;
    },
    {},
  );

  if (!open) {
    return (
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="flex h-8 items-center rounded-md bg-accent px-3 text-xs font-medium text-white hover:bg-accent-hover"
      >
        {label}
      </button>
    );
  }

  return (
    <form
      action={formAction}
      className="surface-card fixed inset-x-4 top-16 z-30 max-h-[80vh] overflow-y-auto p-5 shadow-2xl sm:inset-x-auto sm:right-6 sm:w-[28rem]"
    >
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-medium">{title}</h2>
        <button
          type="button"
          onClick={() => setOpen(false)}
          className="text-xs text-ink-3 hover:text-ink"
          aria-label="Fermer"
        >
          ×
        </button>
      </div>

      <div className="mt-4 space-y-3">{children}</div>

      {state.error && (
        <p className="mt-3 rounded-lg border border-danger/40 bg-danger/10 px-3 py-2 text-2xs text-danger">
          {state.error}
        </p>
      )}

      <button
        type="submit"
        disabled={pending}
        className="mt-4 h-9 w-full rounded-lg bg-accent text-xs font-medium text-white hover:bg-accent-hover disabled:opacity-50"
      >
        {pending ? "Enregistrement…" : submitLabel}
      </button>
    </form>
  );
}

/** Champ de formulaire, étiquette au-dessus. */
export function Field({
  label,
  name,
  type = "text",
  required,
  placeholder,
  hint,
  defaultValue,
}: {
  label: string;
  name: string;
  type?: string;
  required?: boolean;
  placeholder?: string;
  hint?: string;
  defaultValue?: string | number;
}) {
  return (
    <label className="block">
      <span className="mb-1 block text-2xs text-ink-3">
        {label}
        {required && <span className="ml-0.5 text-danger">*</span>}
      </span>
      <input
        name={name}
        type={type}
        required={required}
        placeholder={placeholder}
        defaultValue={defaultValue}
        className="h-9 w-full rounded-lg border border-line bg-surface-0 px-3 text-sm outline-none focus:border-accent"
      />
      {hint && <span className="mt-1 block text-2xs text-ink-3">{hint}</span>}
    </label>
  );
}

/** Liste déroulante. */
export function Select({
  label,
  name,
  options,
  defaultValue,
}: {
  label: string;
  name: string;
  options: { value: string; label: string }[];
  defaultValue?: string;
}) {
  return (
    <label className="block">
      <span className="mb-1 block text-2xs text-ink-3">{label}</span>
      <select
        name={name}
        defaultValue={defaultValue}
        className="h-9 w-full rounded-lg border border-line bg-surface-0 px-2 text-sm outline-none focus:border-accent"
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </label>
  );
}

/** Zone de texte. */
export function TextArea({
  label,
  name,
  rows = 3,
  placeholder,
}: {
  label: string;
  name: string;
  rows?: number;
  placeholder?: string;
}) {
  return (
    <label className="block">
      <span className="mb-1 block text-2xs text-ink-3">{label}</span>
      <textarea
        name={name}
        rows={rows}
        placeholder={placeholder}
        className="w-full rounded-lg border border-line bg-surface-0 px-3 py-2 text-sm outline-none focus:border-accent"
      />
    </label>
  );
}
