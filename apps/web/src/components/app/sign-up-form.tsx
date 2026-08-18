"use client";

import { useActionState } from "react";
import { signUp } from "@/app/actions";

/**
 * Inscription d'un organisme et de son compte propriétaire.
 *
 * Le mot de passe n'a pas de règle de composition affichée : ce que l'API
 * exige, c'est une longueur, et réciter « une majuscule, un chiffre, un
 * caractère spécial » produit des mots de passe plus faibles que trois mots
 * choisis au hasard.
 */
export function SignUpForm({ plan }: { plan?: string }) {
  const [state, action, pending] = useActionState(signUp, undefined);

  return (
    <form action={action} className="mt-8 space-y-3">
      {plan && <input type="hidden" name="plan" value={plan} />}

      <label className="block">
        <span className="mb-1 block text-2xs text-ink-3">Nom de l&apos;organisme</span>
        <input
          name="orgName"
          required
          autoComplete="organization"
          placeholder="Institut Vulcain"
          className="h-11 w-full rounded-lg border border-line bg-surface-0 px-3 text-sm outline-none focus:border-accent"
        />
      </label>

      <div className="grid grid-cols-2 gap-3">
        <label className="block">
          <span className="mb-1 block text-2xs text-ink-3">Prénom</span>
          <input
            name="firstName"
            required
            autoComplete="given-name"
            className="h-11 w-full rounded-lg border border-line bg-surface-0 px-3 text-sm outline-none focus:border-accent"
          />
        </label>
        <label className="block">
          <span className="mb-1 block text-2xs text-ink-3">Nom</span>
          <input
            name="lastName"
            required
            autoComplete="family-name"
            className="h-11 w-full rounded-lg border border-line bg-surface-0 px-3 text-sm outline-none focus:border-accent"
          />
        </label>
      </div>

      <label className="block">
        <span className="mb-1 block text-2xs text-ink-3">Courriel professionnel</span>
        <input
          name="email"
          type="email"
          required
          autoComplete="email"
          className="h-11 w-full rounded-lg border border-line bg-surface-0 px-3 text-sm outline-none focus:border-accent"
        />
      </label>

      <label className="block">
        <span className="mb-1 block text-2xs text-ink-3">Mot de passe</span>
        <input
          name="password"
          type="password"
          required
          autoComplete="new-password"
          minLength={12}
          className="h-11 w-full rounded-lg border border-line bg-surface-0 px-3 text-sm outline-none focus:border-accent"
        />
        <span className="mt-1 block text-2xs text-ink-3">
          Douze caractères au moins. Trois mots choisis au hasard valent mieux
          qu&apos;un mot ponctué de chiffres.
        </span>
      </label>

      {state?.error && (
        <p className="rounded-lg border border-danger/40 bg-danger/10 px-3 py-2 text-xs text-danger">
          {state.error}
        </p>
      )}

      <button
        type="submit"
        disabled={pending}
        className="h-11 w-full rounded-lg bg-accent text-sm font-medium text-white transition-colors duration-[120ms] hover:bg-accent-hover disabled:opacity-60"
      >
        {pending ? "Création…" : "Créer mon organisme"}
      </button>

      <p className="text-2xs text-ink-3">
        Vos données et celles de vos apprenants sont hébergées en France. Un
        export complet reste disponible à tout moment : la réversibilité
        n&apos;est pas une option de la formule.
      </p>
    </form>
  );
}
