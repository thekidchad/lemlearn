"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

/**
 * Choix du mot de passe, à l'entrée dans l'espace apprenant.
 *
 * La session s'ouvre dans la foulée : demander de se reconnecter juste après
 * avoir choisi un mot de passe est une étape que personne ne comprend.
 */
export function AcceptInvitation({ token }: { token: string }) {
  const router = useRouter();
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      const response = await fetch(`/api/invitation/${encodeURIComponent(token)}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ password }),
      });
      const body = (await response.json()) as { error?: string };
      if (!response.ok) throw new Error(body.error ?? "impossible d'ouvrir l'accès");
      router.push("/apprenant");
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "impossible d'ouvrir l'accès");
      setBusy(false);
    }
  };

  return (
    <div className="mt-8 space-y-3">
      <label className="block">
        <span className="mb-1 block text-2xs text-ink-3">Mot de passe</span>
        <input
          type="password"
          autoComplete="new-password"
          minLength={12}
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          className="h-11 w-full rounded-lg border border-line bg-surface-0 px-3 text-sm outline-none focus:border-accent"
        />
        <span className="mt-1 block text-2xs text-ink-3">
          Douze caractères au moins. Trois mots choisis au hasard valent mieux
          qu&apos;un mot ponctué de chiffres.
        </span>
      </label>

      {error && (
        <p className="rounded-lg border border-danger/40 bg-danger/10 px-3 py-2 text-xs text-danger">
          {error}
        </p>
      )}

      <button
        type="button"
        onClick={submit}
        disabled={busy || password.length < 12}
        className="h-11 w-full rounded-lg bg-accent text-sm font-medium text-white transition-colors duration-[120ms] hover:bg-accent-hover disabled:opacity-50"
      >
        {busy ? "Ouverture…" : "Entrer dans mon espace"}
      </button>
    </div>
  );
}
