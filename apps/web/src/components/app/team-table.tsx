"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

/**
 * Les accès à l'espace de l'organisme.
 *
 * Un organisme n'était qu'un seul compte : le propriétaire, et personne
 * d'autre. Pas d'assistante, pas de formateur — et donc personne à qui confier
 * un dossier, ni à qui faire contresigner une feuille d'émargement.
 *
 * Un accès se suspend, il ne se supprime pas. Les actions d'un collaborateur
 * restent au journal sous son nom : effacer le compte laisserait des
 * événements signés par un identifiant que plus rien n'explique.
 */
export interface Membre {
  id: string;
  email: string;
  firstName: string;
  lastName: string;
  role: string;
  roleLabel: string;
  disabled: boolean;
  pending: boolean;
  lastLoginAt?: string;
}

const ROLES = [
  {
    valeur: "owner",
    label: "Propriétaire",
    peut: "Tout, y compris les accès, la facturation et l'identité juridique.",
  },
  {
    valeur: "admin",
    label: "Administrateur",
    peut: "Les stagiaires, le catalogue, les dossiers et les pièces probatoires.",
  },
  {
    valeur: "trainer",
    label: "Formateur",
    peut: "Ses sessions : émargement, évaluations, suivi de ses stagiaires.",
  },
] as const;

export function TeamTable({
  membres,
  moi,
  peutGerer,
}: {
  membres: Membre[];
  moi: string;
  peutGerer: boolean;
}) {
  const router = useRouter();
  const [ouvert, setOuvert] = useState(false);
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [lien, setLien] = useState<{ url: string; sentTo?: string; warning?: string } | null>(
    null,
  );

  const inviter = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setError(null);
    setBusy("invitation");
    try {
      const response = await fetch("/api/equipe", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          email: String(form.get("email") ?? ""),
          firstName: String(form.get("firstName") ?? ""),
          lastName: String(form.get("lastName") ?? ""),
          role: String(form.get("role") ?? "admin"),
        }),
      });
      const body = (await response.json()) as {
        invitationUrl?: string;
        sentTo?: string;
        warning?: string;
        error?: string;
      };
      if (!response.ok || !body.invitationUrl) {
        throw new Error(body.error ?? "invitation refusée");
      }
      setLien({ url: body.invitationUrl, sentTo: body.sentTo, warning: body.warning });
      setOuvert(false);
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "invitation refusée");
    } finally {
      setBusy(null);
    }
  };

  const relancer = async (membre: Membre) => {
    setError(null);
    setBusy(membre.id);
    try {
      const response = await fetch("/api/equipe", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          email: membre.email,
          firstName: membre.firstName,
          lastName: membre.lastName,
          role: membre.role,
        }),
      });
      const body = (await response.json()) as {
        invitationUrl?: string;
        sentTo?: string;
        warning?: string;
        error?: string;
      };
      if (!response.ok || !body.invitationUrl) throw new Error(body.error ?? "relance refusée");
      setLien({ url: body.invitationUrl, sentTo: body.sentTo, warning: body.warning });
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "relance refusée");
    } finally {
      setBusy(null);
    }
  };

  const modifier = async (membre: Membre, patch: { role?: string; disabled?: boolean }) => {
    setError(null);
    setBusy(membre.id);
    try {
      const response = await fetch(`/api/equipe/${membre.id}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(patch),
      });
      const body = (await response.json()) as { error?: string };
      if (!response.ok) throw new Error(body.error ?? "changement refusé");
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "changement refusé");
    } finally {
      setBusy(null);
    }
  };

  return (
    <section className="surface-card overflow-hidden">
      <header className="flex flex-wrap items-center gap-3 border-b border-line px-5 py-3">
        <h2 className="text-sm font-medium">Accès à l&apos;espace</h2>
        <span className="font-mono text-2xs text-ink-3" data-numeric>
          {membres.filter((membre) => !membre.disabled).length} actif
          {membres.filter((membre) => !membre.disabled).length > 1 ? "s" : ""} sur{" "}
          {membres.length}
        </span>
        {peutGerer && (
          <button
            type="button"
            className="btn-primary ml-auto"
            onClick={() => {
              setLien(null);
              setOuvert((etat) => !etat);
            }}
          >
            {ouvert ? "Annuler" : "Ouvrir un accès"}
          </button>
        )}
      </header>

      {error && <p className="px-5 py-2 text-xs text-danger">{error}</p>}

      {lien && (
        <div className="border-b border-line bg-surface-2 px-5 py-4">
          <p className="text-xs text-ink-2">
            {lien.sentTo
              ? `L'invitation est partie à ${lien.sentTo}.`
              : "L'accès est ouvert."}{" "}
            {lien.warning}
          </p>
          {/* Le lien est montré quoi qu'il arrive : si le courriel n'est pas
              parti, c'est le seul rattrapage — réinviter échouerait, l'adresse
              étant désormais réservée. */}
          <p className="mt-2 font-mono text-2xs break-all text-ink-3">{lien.url}</p>
        </div>
      )}

      {ouvert && (
        <form onSubmit={inviter} className="border-b border-line bg-surface-2 px-5 py-4">
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="block">
              <span className="eyebrow">Prénom</span>
              <input name="firstName" className="field mt-1.5" />
            </label>
            <label className="block">
              <span className="eyebrow">Nom</span>
              <input name="lastName" className="field mt-1.5" />
            </label>
            <label className="block sm:col-span-2">
              <span className="eyebrow">Courriel</span>
              <input name="email" type="email" required className="field mt-1.5" />
            </label>
            <label className="block sm:col-span-2">
              <span className="eyebrow">Rôle</span>
              <select name="role" defaultValue="admin" className="field mt-1.5">
                {ROLES.map((role) => (
                  <option key={role.valeur} value={role.valeur}>
                    {role.label} — {role.peut}
                  </option>
                ))}
              </select>
            </label>
          </div>
          <p className="mt-3 text-2xs text-ink-3">
            Aucun mot de passe n&apos;est fabriqué ici : la personne choisira le
            sien par le lien reçu.
          </p>
          <button type="submit" className="btn-primary mt-4" disabled={busy === "invitation"}>
            {busy === "invitation" ? "Envoi…" : "Ouvrir l'accès et inviter"}
          </button>
        </form>
      )}

      <ul className="divide-y divide-line/60">
        {membres.map((membre) => (
          <li key={membre.id} className="flex flex-wrap items-center gap-x-4 gap-y-2 px-5 py-3">
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm">
                {[membre.firstName, membre.lastName].filter(Boolean).join(" ") || membre.email}
                {membre.id === moi && <span className="ml-2 text-2xs text-ink-3">(vous)</span>}
              </p>
              <p className="truncate font-mono text-2xs text-ink-3">
                {membre.email}
                {membre.lastLoginAt ? ` · dernière entrée le ${membre.lastLoginAt}` : ""}
              </p>
            </div>

            <span
              className={`shrink-0 rounded px-1.5 py-0.5 text-2xs ${
                membre.pending
                  ? "bg-warn/15 text-warn"
                  : membre.disabled
                    ? "border border-line-strong text-ink-3"
                    : "bg-ok/15 text-ok"
              }`}
            >
              {membre.pending ? "en attente" : membre.disabled ? "suspendu" : "actif"}
            </span>

            {peutGerer ? (
              <div className="flex shrink-0 items-center gap-2">
                <select
                  aria-label={`Rôle de ${membre.email}`}
                  defaultValue={membre.role}
                  disabled={busy !== null}
                  onChange={(event) => modifier(membre, { role: event.target.value })}
                  className="field h-8 w-40 text-xs"
                >
                  {ROLES.map((role) => (
                    <option key={role.valeur} value={role.valeur}>
                      {role.label}
                    </option>
                  ))}
                </select>
                {/* Un compte jamais activé se relance : le « rétablir » le
                    marquerait actif sans le rendre utilisable, la connexion
                    échouant contre une empreinte de mot de passe vide. */}
                {membre.pending ? (
                  <button
                    type="button"
                    className="btn-ghost"
                    disabled={busy !== null}
                    onClick={() => relancer(membre)}
                  >
                    Renvoyer l&apos;invitation
                  </button>
                ) : (
                  <button
                    type="button"
                    className="btn-ghost"
                    disabled={busy !== null || membre.id === moi}
                    title={
                      membre.id === moi
                        ? "Vous ne pouvez pas suspendre votre propre accès."
                        : undefined
                    }
                    onClick={() => modifier(membre, { disabled: !membre.disabled })}
                  >
                    {membre.disabled ? "Rétablir" : "Suspendre"}
                  </button>
                )}
              </div>
            ) : (
              <span className="shrink-0 text-2xs text-ink-3">{membre.roleLabel}</span>
            )}
          </li>
        ))}
      </ul>

      <p className="border-t border-line px-5 py-3 text-2xs text-ink-3">
        Un accès se suspend, il ne se supprime pas : les actions d&apos;un
        collaborateur restent au journal sous son nom, et effacer le compte
        laisserait des événements que plus rien n&apos;explique. Chaque ouverture
        et chaque retrait y sont d&apos;ailleurs inscrits.
      </p>
    </section>
  );
}
