"use client";

import { useState } from "react";
import type { Plan } from "@/app/(app)/admin/page";

/**
 * Choix de la formule.
 *
 * Le bouton n'apparaît que si le paiement en ligne est ouvert : proposer un
 * parcours qui échoue au clic est pire que de renvoyer vers un contact.
 */
export function SubscriptionPlans({
  plans,
  current,
  selfServe,
}: {
  plans: Plan[];
  current: string;
  selfServe: boolean;
}) {
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const subscribe = async (code: string) => {
    setBusy(code);
    setError(null);
    try {
      const response = await fetch("/api/abonnement", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ plan: code }),
      });
      const body = (await response.json()) as { url?: string; error?: string };
      if (!response.ok || !body.url) throw new Error(body.error ?? "paiement indisponible");
      // assign() plutôt qu'une affectation : le lint interdit d'écrire dans
      // un objet global, et la navigation est de toute façon plus explicite.
      window.location.assign(body.url);
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "paiement indisponible");
      setBusy(null);
    }
  };

  return (
    <div className="mt-6">
      <div className="grid gap-3 sm:grid-cols-3">
        {plans
          .filter((plan) => plan.priceCents > 0)
          .map((plan) => (
            <article
              key={plan.code}
              className={`surface-card flex flex-col p-4 ${
                plan.code === current ? "ring-1 ring-accent/40" : ""
              }`}
            >
              <p className="text-sm font-medium">{plan.label}</p>
              <p className="mt-1.5 text-lg font-semibold tracking-[-0.03em]" data-numeric>
                {plan.priceCents / 100} €
                <span className="ml-1 text-2xs font-normal text-ink-3">/ mois HT</span>
              </p>
              <p className="mt-1.5 flex-1 text-2xs text-ink-2">{plan.description}</p>

              <ul className="mt-3 space-y-1 font-mono text-2xs text-ink-3">
                <li>{plan.maxLearners} apprenants</li>
                <li>{plan.maxSignatures} signatures / mois</li>
                <li>
                  {plan.maxVideoHours} h de vidéo · {plan.maxStorageGb} Go
                </li>
              </ul>

              {plan.code === current ? (
                <p className="mt-4 text-2xs text-accent-ink">Formule en cours</p>
              ) : selfServe ? (
                <button
                  type="button"
                  onClick={() => subscribe(plan.code)}
                  disabled={busy !== null}
                  className="mt-4 h-9 rounded-lg border border-line-strong text-xs font-medium hover:border-accent disabled:opacity-50"
                >
                  {busy === plan.code ? "Ouverture…" : "Choisir"}
                </button>
              ) : (
                <a
                  href="mailto:contact@lemlearn.fr"
                  className="mt-4 flex h-9 items-center justify-center rounded-lg border border-line text-xs text-ink-2 hover:border-accent hover:text-ink"
                >
                  Nous écrire
                </a>
              )}
            </article>
          ))}
      </div>

      {error && (
        <p className="mt-3 rounded-lg border border-danger/40 bg-danger/10 px-3 py-2 text-xs text-danger">
          {error}
        </p>
      )}
    </div>
  );
}
