"use client";

export interface Question {
  id: string;
  type: "single" | "multiple" | "boolean" | "likert" | "numeric" | "text";
  prompt: string;
  options?: { id: string; label: string }[];
  required: boolean;
  points?: number;
  explanation?: string;
  correct?: string[];
}

/**
 * Le corps d'une question, quelle que soit sa forme.
 *
 * Partagé par le questionnaire de satisfaction et par le contrôle après
 * module : deux rendus séparés finiraient par diverger, et une échelle de
 * Likert qui ne se comporte pas pareil selon l'écran est le genre de détail
 * qui fait douter du reste.
 */
export function QuestionField({
  question,
  values,
  onChange,
  disabled,
}: {
  question: Question;
  values: string[];
  onChange: (values: string[]) => void;
  disabled?: boolean;
}) {
  const toggle = (optionId: string) =>
    onChange(
      values.includes(optionId)
        ? values.filter((value) => value !== optionId)
        : [...values, optionId],
    );

  switch (question.type) {
    case "single":
    case "boolean":
      return (
        <>
          {question.options?.map((option) => (
            <Choice
              key={option.id}
              name={question.id}
              label={option.label}
              type="radio"
              checked={values.includes(option.id)}
              disabled={disabled}
              onChange={() => onChange([option.id])}
            />
          ))}
        </>
      );

    case "multiple":
      return (
        <>
          {question.options?.map((option) => (
            <Choice
              key={option.id}
              name={question.id}
              label={option.label}
              type="checkbox"
              checked={values.includes(option.id)}
              disabled={disabled}
              onChange={() => toggle(option.id)}
            />
          ))}
        </>
      );

    case "likert":
      return (
        <>
          <div className="flex gap-1.5">
            {[1, 2, 3, 4, 5].map((value) => {
              const selected = values[0] === String(value);
              return (
                <button
                  key={value}
                  type="button"
                  disabled={disabled}
                  onClick={() => onChange([String(value)])}
                  aria-pressed={selected}
                  className={`h-10 flex-1 rounded-lg border text-sm transition-colors duration-[120ms] disabled:opacity-60 ${
                    selected
                      ? "border-accent bg-accent/15 text-ink"
                      : "border-line text-ink-2 hover:border-line-strong"
                  }`}
                  data-numeric
                >
                  {value}
                </button>
              );
            })}
          </div>
          <p className="mt-2 flex justify-between text-2xs text-ink-3">
            <span>Pas du tout</span>
            <span>Tout à fait</span>
          </p>
        </>
      );

    case "numeric":
      return (
        <input
          type="number"
          inputMode="decimal"
          disabled={disabled}
          defaultValue={values[0] ?? ""}
          onChange={(event) => onChange([event.target.value])}
          className="h-10 w-40 rounded-lg border border-line bg-surface-0 px-3 text-sm outline-none focus:border-accent disabled:opacity-60"
          data-numeric
        />
      );

    case "text":
      return (
        <textarea
          rows={3}
          disabled={disabled}
          defaultValue={values[0] ?? ""}
          onChange={(event) => onChange([event.target.value])}
          placeholder="Votre réponse"
          className="w-full rounded-lg border border-line bg-surface-0 px-3 py-2 text-sm outline-none focus:border-accent disabled:opacity-60"
        />
      );
  }
}

function Choice({
  name,
  label,
  type,
  checked,
  disabled,
  onChange,
}: {
  name: string;
  label: string;
  type: "radio" | "checkbox";
  checked: boolean;
  disabled?: boolean;
  onChange: () => void;
}) {
  return (
    <label className="flex cursor-pointer items-center gap-2.5 rounded-lg px-1 py-2 text-sm text-ink-2 hover:text-ink">
      <input
        type={type}
        name={name}
        checked={checked}
        disabled={disabled}
        onChange={onChange}
        className="size-4 accent-[var(--color-accent)]"
      />
      {label}
    </label>
  );
}
