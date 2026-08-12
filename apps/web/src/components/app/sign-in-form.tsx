"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";
import { signIn } from "@/app/actions";
import { Button } from "@/components/ui/button";

/**
 * Formulaire de connexion.
 *
 * Le seul composant client de l'application : il lui faut l'état de
 * soumission et le message d'erreur renvoyé par l'action. Tout le reste est
 * rendu sur le serveur.
 */
export function SignInForm() {
  const [state, action] = useActionState(signIn, undefined);

  return (
    <form action={action} className="mt-8 space-y-4">
      <Field
        label="Adresse e-mail"
        name="email"
        type="email"
        autoComplete="username"
        placeholder="marie@votre-organisme.fr"
        required
      />
      <Field
        label="Mot de passe"
        name="password"
        type="password"
        autoComplete="current-password"
        required
      />

      {state?.error && (
        <p
          role="alert"
          className="rounded-lg border border-bad/40 bg-bad/10 px-3 py-2 text-xs text-bad"
        >
          {state.error}
        </p>
      )}

      <Submit />
    </form>
  );
}

function Submit() {
  const { pending } = useFormStatus();
  return (
    <Button type="submit" size="lg" className="w-full" disabled={pending}>
      {pending ? "Connexion…" : "Se connecter"}
    </Button>
  );
}

function Field({
  label,
  name,
  ...props
}: { label: string; name: string } & React.ComponentProps<"input">) {
  return (
    <label className="block">
      <span className="mb-1.5 block text-2xs tracking-wide text-ink-3 uppercase">{label}</span>
      <input
        name={name}
        className="h-10 w-full rounded-lg border border-line bg-surface-1 px-3 text-sm text-ink transition-colors duration-[120ms] outline-none placeholder:text-ink-3 hover:border-line-strong focus:border-accent"
        {...props}
      />
    </label>
  );
}
