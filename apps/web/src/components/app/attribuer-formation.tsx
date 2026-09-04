"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";

/**
 * Attribuer une formation, depuis la fiche de la personne.
 *
 * C'est le geste que fait un organisme après avoir créé un stagiaire, et il
 * n'existait pas : il fallait quitter la fiche, aller dans Sessions, trouver
 * la bonne, et l'inscrire de là. Le bouton était donc à l'endroit où personne
 * ne le cherchait — sur l'objet qu'on n'a pas encore sous les yeux.
 *
 * Ici, on choisit une formation du catalogue et c'est tout. Le reste est
 * assemblé par le serveur : la session est reprise si une est ouverte, ouverte
 * sinon, et le dossier — celui qui portera la convention et les preuves — est
 * créé du même mouvement.
 */
interface Formation {
  id: string;
  title: string;
  durationHours: number;
  priceHT: number;
  published: boolean;
}

interface Typologie {
  code: string;
  label: string;
}

const ORIGINES = [
  { code: "company", label: "Entreprise (hors OPCO)" },
  { code: "opco", label: "Opérateur de compétences" },
  { code: "public", label: "Fonds publics (État, Régions, France Travail)" },
  { code: "individual", label: "Le stagiaire lui-même" },
  { code: "subcontract", label: "Sous-traitance d'un autre organisme" },
  { code: "other", label: "Autre" },
] as const;

export function AttribuerFormation({
  contactId,
  formations,
  stagiaires,
}: {
  contactId: string;
  formations: Formation[];
  /** Les intitulés du cadre E du bilan, servis par l'API. */
  stagiaires: Typologie[];
}) {
  const router = useRouter();
  const [ouvert, setOuvert] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fait, setFait] = useState<{
    course: string;
    reference?: string;
    deja?: boolean;
  } | null>(null);

  const publiees = formations.filter((formation) => formation.published);

  const attribuer = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    setError(null);
    setBusy(true);
    try {
      const response = await fetch(`/api/contacts/${contactId}/inscription`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          courseId: String(data.get("courseId") ?? ""),
          startsAt: String(data.get("startsAt") ?? ""),
          endsAt: String(data.get("endsAt") ?? ""),
          traineeType: String(data.get("traineeType") ?? ""),
          funding: String(data.get("funding") ?? ""),
          priceHT: Number(data.get("priceHT") ?? 0),
        }),
      });
      const body = (await response.json()) as {
        course?: { title: string };
        file?: { reference: string };
        deja?: boolean;
        error?: string;
      };
      if (!response.ok) throw new Error(body.error ?? "inscription refusée");
      setFait({
        course: body.course?.title ?? "",
        reference: body.file?.reference,
        deja: Boolean(body.deja),
      });
      setOuvert(false);
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "inscription refusée");
    } finally {
      setBusy(false);
    }
  };

  // Le catalogue vide se dit ici plutôt que de laisser un menu déroulant sans
  // option : c'est la première chose à faire, et rien ne l'indiquait.
  if (publiees.length === 0) {
    return (
      <div className="surface-card p-5">
        <h2 className="text-sm font-medium">Attribuer une formation</h2>
        <p className="mt-2 text-xs text-ink-2">
          Votre catalogue ne contient aucune formation publiée. C&apos;est par là
          qu&apos;il faut commencer : on décrit d&apos;abord ce qu&apos;on
          propose, puis on y inscrit des stagiaires.
        </p>
        <Link href="/catalogue" className="btn-secondary mt-4 inline-flex">
          Aller au catalogue
        </Link>
      </div>
    );
  }

  return (
    <section className="surface-card overflow-hidden">
      <header className="flex flex-wrap items-center gap-3 px-5 py-3">
        <h2 className="text-sm font-medium">Attribuer une formation</h2>
        <button
          type="button"
          className="btn-primary ml-auto"
          onClick={() => {
            setFait(null);
            setOuvert((etat) => !etat);
          }}
        >
          {ouvert ? "Annuler" : "Attribuer une formation"}
        </button>
      </header>

      {error && <p className="px-5 pb-3 text-xs text-danger">{error}</p>}

      {fait && (
        <p
          className={`border-t border-line px-5 py-3 text-xs ${
            fait.deja ? "bg-surface-2 text-ink-2" : "bg-ok/10 text-ok"
          }`}
        >
          {fait.deja
            ? `Déjà inscrit à « ${fait.course} » — rien n'a été dupliqué.`
            : `Inscrit à « ${fait.course} ».`}
          {fait.reference ? ` Dossier ${fait.reference}.` : ""}
        </p>
      )}

      {ouvert && (
        <form onSubmit={attribuer} className="border-t border-line bg-surface-2 px-5 py-4">
          <label className="block">
            <span className="eyebrow">Formation du catalogue</span>
            <select name="courseId" required className="field mt-1.5">
              {publiees.map((formation) => (
                <option key={formation.id} value={formation.id}>
                  {formation.title} — {formation.durationHours} h
                </option>
              ))}
            </select>
          </label>

          <div className="mt-3 grid gap-3 sm:grid-cols-2">
            <label className="block">
              <span className="eyebrow">Début</span>
              <input name="startsAt" type="date" className="field mt-1.5" />
            </label>
            <label className="block">
              <span className="eyebrow">Fin</span>
              <input name="endsAt" type="date" className="field mt-1.5" />
            </label>
            <label className="block">
              <span className="eyebrow">Nature du stagiaire</span>
              <select name="traineeType" defaultValue="" className="field mt-1.5">
                <option value="">À préciser</option>
                {stagiaires.map((option) => (
                  <option key={option.code} value={option.code}>
                    {option.label}
                  </option>
                ))}
              </select>
            </label>
            <label className="block">
              <span className="eyebrow">Qui finance</span>
              <select name="funding" defaultValue="" className="field mt-1.5">
                <option value="">À préciser</option>
                {ORIGINES.map((origine) => (
                  <option key={origine.code} value={origine.code}>
                    {origine.label}
                  </option>
                ))}
              </select>
            </label>
            <label className="block sm:col-span-2">
              <span className="eyebrow">Prix HT</span>
              <input
                name="priceHT"
                type="number"
                step="0.01"
                placeholder="Celui du catalogue si vide"
                className="field mt-1.5"
              />
            </label>
          </div>

          <p className="mt-3 text-2xs text-ink-3">
            Une session ouverte de cette formation est reprise ; sinon elle est
            créée. Un dossier est ouvert dans la foulée — c&apos;est lui qui
            portera la convention, les émargements et l&apos;export probatoire.
          </p>

          <button type="submit" className="btn-primary mt-4" disabled={busy}>
            {busy ? "Inscription…" : "Inscrire à cette formation"}
          </button>
        </form>
      )}
    </section>
  );
}
