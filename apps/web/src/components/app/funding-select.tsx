"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

/**
 * Origine des fonds d'un dossier.
 *
 * Les catégories sont celles du cadre C du bilan pédagogique et financier :
 * s'en écarter obligerait à retraduire chaque ligne au moment de la
 * déclaration, c'est-à-dire au pire moment.
 *
 * On le demande ici, sur le dossier, parce que c'est le seul renseignement du
 * bilan qu'on ne peut pas reconstituer après coup : douze mois plus tard,
 * personne ne se souvient si la formation a été prise en charge par l'OPCO ou
 * payée par l'entreprise.
 */
const SOURCES: { valeur: string; label: string }[] = [
  { valeur: "", label: "Non renseignée" },
  { valeur: "entreprise", label: "Entreprise (hors OPCO)" },
  { valeur: "opco", label: "Opérateur de compétences" },
  { valeur: "public", label: "Fonds publics (État, Région, France Travail)" },
  { valeur: "particulier", label: "Le stagiaire lui-même" },
  { valeur: "sous-traitance", label: "Autre organisme (sous-traitance)" },
  { valeur: "autre", label: "Autre" },
];

export function FundingSelect({
  fileId,
  current,
}: {
  fileId: string;
  current?: string;
}) {
  const router = useRouter();
  const [valeur, setValeur] = useState(current ?? "");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const save = async (source: string) => {
    setValeur(source);
    setError(null);
    setBusy(true);
    try {
      const response = await fetch(`/api/dossiers/${fileId}/financement`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ funding: source }),
      });
      if (!response.ok) throw new Error("enregistrement refusé");
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "enregistrement impossible");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div>
      <label className="block">
        <span className="eyebrow">Origine des fonds</span>
        <select
          value={valeur}
          disabled={busy}
          onChange={(event) => void save(event.target.value)}
          className="field mt-1.5"
        >
          {SOURCES.map((source) => (
            <option key={source.valeur} value={source.valeur}>
              {source.label}
            </option>
          ))}
        </select>
      </label>
      <p className="mt-1 text-2xs text-ink-3">
        Reprise dans votre bilan pédagogique et financier annuel.
      </p>
      {error && <p className="mt-1 text-2xs text-danger">{error}</p>}
    </div>
  );
}
