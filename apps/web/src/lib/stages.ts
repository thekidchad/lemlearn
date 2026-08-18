/**
 * Étapes du pipeline et complétude du dossier.
 *
 * Ce module ne dépend de rien du serveur : il est importé par des composants
 * client, et `lib/api` tire `next/headers`, qui n'y a pas sa place.
 */
export type Stage = "prospect" | "quote" | "agreement" | "in_training" | "closed" | "lost";

export interface ProofStatus {
  expected: number;
  present: number;
  missing?: string[];
}

export const STAGES: { key: Stage; label: string }[] = [
  { key: "prospect", label: "Prospect" },
  { key: "quote", label: "Devis" },
  { key: "agreement", label: "Convention" },
  { key: "in_training", label: "En formation" },
  { key: "closed", label: "Clôturé" },
];

/** proofPercent est la complétude du dossier de preuve, en pourcentage. */
export function proofPercent(proof: ProofStatus): number {
  if (!proof?.expected) return 0;
  return Math.round((proof.present * 100) / proof.expected);
}
