"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

/**
 * Le compte, vu par celui à qui il appartient.
 *
 * Le même écran sert les trois espaces — l'organisme, l'équipe et le stagiaire
 * — parce que c'est la même chose qu'on y fait : corriger son nom, changer son
 * mot de passe, poser sa photo. Ce qui diffère d'un rôle à l'autre, ce sont les
 * pièces qu'on retrouve à côté, et elles sont passées en paramètre.
 */
export interface Compte {
  id: string;
  email: string;
  firstName: string;
  lastName: string;
  role: string;
  roleLabel?: string;
  photoUrl?: string;
  lastLoginAt?: string;
}

export function ProfilCompte({
  compte,
  orgName,
  impersonated,
  liens,
}: {
  compte: Compte;
  orgName: string;
  /** Une session ouverte par l'équipe : le mot de passe n'est pas le sien. */
  impersonated: boolean;
  /** Ce qu'on retrouve depuis son compte, selon le rôle. */
  liens: { href: string; label: string; aide: string }[];
}) {
  const router = useRouter();
  const [photo, setPhoto] = useState(compte.photoUrl ?? "");
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [dit, setDit] = useState<string | null>(null);

  // Le dépôt en trois temps : on signe, le navigateur écrit dans le
  // compartiment, puis on rattache. La photo ne transite jamais par l'API.
  const deposer = async (file: File) => {
    setError(null);
    setDit(null);
    setBusy("photo");
    try {
      const reserve = await fetch("/api/profil/photo", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ contentType: file.type }),
      });
      const reserved = (await reserve.json()) as {
        uploadUrl?: string;
        key?: string;
        error?: string;
      };
      if (!reserve.ok || !reserved.uploadUrl || !reserved.key) {
        throw new Error(reserved.error ?? "dépôt refusé");
      }

      const put = await fetch(reserved.uploadUrl, {
        method: "PUT",
        headers: { "Content-Type": file.type },
        body: file,
      });
      if (!put.ok) throw new Error(`le dépôt a échoué (${put.status})`);

      const attach = await fetch("/api/profil/photo", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ key: reserved.key }),
      });
      const attached = (await attach.json()) as { photoUrl?: string; error?: string };
      if (!attach.ok) throw new Error(attached.error ?? "enregistrement refusé");

      // L'aperçu lit le fichier local : l'objet vient d'être écrit sous une
      // adresse que le navigateur n'a pas encore.
      setPhoto(URL.createObjectURL(file));
      setDit("Photo mise à jour.");
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "dépôt impossible");
    } finally {
      setBusy(null);
    }
  };

  const retirer = async () => {
    setError(null);
    setBusy("photo");
    try {
      const response = await fetch("/api/profil/photo", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ key: "" }),
      });
      if (!response.ok) throw new Error("retrait refusé");
      setPhoto("");
      setDit("Photo retirée.");
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "retrait impossible");
    } finally {
      setBusy(null);
    }
  };

  const enregistrerNom = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    setError(null);
    setDit(null);
    setBusy("nom");
    try {
      const response = await fetch("/api/profil", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          firstName: String(data.get("firstName") ?? ""),
          lastName: String(data.get("lastName") ?? ""),
        }),
      });
      const body = (await response.json()) as { error?: string };
      if (!response.ok) throw new Error(body.error ?? "enregistrement refusé");
      setDit("Nom enregistré.");
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "enregistrement refusé");
    } finally {
      setBusy(null);
    }
  };

  const changerMotDePasse = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    setError(null);
    setDit(null);
    setBusy("motdepasse");
    try {
      const response = await fetch("/api/profil/mot-de-passe", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          current: String(data.get("current") ?? ""),
          next: String(data.get("next") ?? ""),
        }),
      });
      if (!response.ok) {
        const body = (await response.json().catch(() => ({}))) as { error?: string };
        throw new Error(body.error ?? "changement refusé");
      }
      form.reset();
      setDit("Mot de passe changé.");
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "changement refusé");
    } finally {
      setBusy(null);
    }
  };

  const initiales =
    `${compte.firstName.charAt(0)}${compte.lastName.charAt(0)}`.toUpperCase() || "—";

  return (
    <div className="space-y-6">
      {error && <p className="text-xs text-danger">{error}</p>}
      {dit && <p className="text-xs text-ok">{dit}</p>}

      <section className="surface-card p-5">
        <div className="flex flex-wrap items-center gap-4">
          {photo ? (
            // Une balise <img> et non next/image : la source est un
            // compartiment S3 dont l'hôte change d'un environnement à l'autre,
            // et l'optimiseur refuse un domaine qu'il ne connaît pas.
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={photo}
              alt=""
              className="size-16 shrink-0 rounded-full object-cover"
            />
          ) : (
            <span
              aria-hidden
              className="flex size-16 shrink-0 items-center justify-center rounded-full bg-accent-dim text-lg font-medium text-accent-ink"
            >
              {initiales}
            </span>
          )}

          <div className="min-w-0 flex-1">
            <p className="text-sm font-medium">
              {[compte.firstName, compte.lastName].filter(Boolean).join(" ") || compte.email}
            </p>
            <p className="truncate font-mono text-2xs text-ink-3">
              {compte.email}
              {compte.roleLabel ? ` · ${compte.roleLabel}` : ""} · {orgName}
            </p>
            {compte.lastLoginAt && (
              <p className="mt-0.5 font-mono text-2xs text-ink-3">
                dernière entrée le {new Date(compte.lastLoginAt).toLocaleString("fr-FR")}
              </p>
            )}
          </div>

          <div className="flex shrink-0 flex-wrap items-center gap-2">
            <label className="btn-secondary cursor-pointer">
              {busy === "photo" ? "Dépôt…" : photo ? "Changer la photo" : "Ajouter une photo"}
              <input
                type="file"
                accept="image/jpeg,image/png,image/webp"
                className="hidden"
                disabled={busy !== null}
                onChange={(event) => {
                  const file = event.target.files?.[0];
                  event.target.value = "";
                  if (file) void deposer(file);
                }}
              />
            </label>
            {photo && (
              <button
                type="button"
                className="btn-ghost"
                disabled={busy !== null}
                onClick={retirer}
              >
                Retirer
              </button>
            )}
          </div>
        </div>
      </section>

      <section className="surface-card overflow-hidden">
        <h2 className="border-b border-line px-5 py-3 text-sm font-medium">Mon identité</h2>
        <form onSubmit={enregistrerNom} className="px-5 py-4">
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="block">
              <span className="eyebrow">Prénom</span>
              <input name="firstName" defaultValue={compte.firstName} className="field mt-1.5" />
            </label>
            <label className="block">
              <span className="eyebrow">Nom</span>
              <input name="lastName" defaultValue={compte.lastName} className="field mt-1.5" />
            </label>
          </div>
          <p className="mt-3 text-2xs text-ink-3">
            {/* L'adresse n'est pas modifiable ici, et c'est délibéré : elle est
                la clé de connexion et la réservation du compte. */}
            L&apos;adresse de connexion est <span className="font-mono">{compte.email}</span>.
            Elle ne se change pas depuis ici : c&apos;est elle qui identifie le
            compte, et se tromper en la saisissant fermerait la porte.
          </p>
          <button type="submit" className="btn-primary mt-4" disabled={busy === "nom"}>
            {busy === "nom" ? "Enregistrement…" : "Enregistrer"}
          </button>
        </form>
      </section>

      <section className="surface-card overflow-hidden">
        <h2 className="border-b border-line px-5 py-3 text-sm font-medium">Mot de passe</h2>
        {impersonated ? (
          <p className="px-5 py-4 text-xs text-ink-2">
            Vous êtes dans le compte de quelqu&apos;un d&apos;autre : son mot de
            passe lui appartient, et le changer d&apos;ici l&apos;enfermerait
            dehors.
          </p>
        ) : (
          <form onSubmit={changerMotDePasse} className="px-5 py-4">
            <div className="grid gap-3 sm:grid-cols-2">
              <label className="block">
                <span className="eyebrow">Mot de passe actuel</span>
                <input
                  name="current"
                  type="password"
                  required
                  autoComplete="current-password"
                  className="field mt-1.5"
                />
              </label>
              <label className="block">
                <span className="eyebrow">Nouveau mot de passe</span>
                <input
                  name="next"
                  type="password"
                  required
                  minLength={12}
                  autoComplete="new-password"
                  className="field mt-1.5"
                />
              </label>
            </div>
            <p className="mt-3 text-2xs text-ink-3">
              Douze caractères au moins. Trois mots choisis au hasard valent
              mieux qu&apos;un mot ponctué de chiffres. L&apos;actuel est demandé
              même si vous êtes déjà connecté : un poste laissé ouvert suffirait
              sinon à verrouiller votre compte.
            </p>
            <button type="submit" className="btn-primary mt-4" disabled={busy === "motdepasse"}>
              {busy === "motdepasse" ? "Changement…" : "Changer le mot de passe"}
            </button>
          </form>
        )}
      </section>

      {liens.length > 0 && (
        <section className="surface-card overflow-hidden">
          <h2 className="border-b border-line px-5 py-3 text-sm font-medium">
            Retrouver mes pièces
          </h2>
          <ul className="divide-y divide-line/60">
            {liens.map((lien) => (
              <li key={lien.href}>
                <a
                  href={lien.href}
                  className="flex items-center justify-between gap-4 px-5 py-3 transition-colors duration-[120ms] hover:bg-surface-2"
                >
                  <span className="min-w-0">
                    <span className="block text-sm">{lien.label}</span>
                    <span className="block text-2xs text-ink-3">{lien.aide}</span>
                  </span>
                  <span aria-hidden className="shrink-0 text-ink-3">
                    →
                  </span>
                </a>
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}
