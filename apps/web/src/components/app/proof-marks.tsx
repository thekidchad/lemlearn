import type { ProofStatus } from "@/lib/stages";

/**
 * Bordereau des pièces d'un dossier.
 *
 * Un dossier probatoire n'est pas un pourcentage : c'est une liste de pièces
 * qu'on possède ou qu'on ne possède pas. Treize cases, treize pièces — on voit
 * d'un coup combien manquent, et le survol dit lesquelles. La jauge continue
 * qu'on met d'ordinaire ici laisse croire à un remplissage progressif, alors
 * qu'aucune pièce n'est à moitié présente.
 */
export function ProofMarks({
  proof,
  size = "sm",
}: {
  proof: ProofStatus;
  size?: "sm" | "md";
}) {
  const total = Math.max(proof.expected, 1);
  const present = Math.min(proof.present, total);
  const missing = proof.missing ?? [];

  return (
    <span
      className="inline-flex items-center gap-1.5"
      title={
        missing.length > 0
          ? `Manque : ${missing.join(", ")}`
          : "Dossier complet : toutes les pièces sont là"
      }
    >
      <span className={`flex ${size === "md" ? "gap-1" : "gap-0.5"}`} aria-hidden>
        {Array.from({ length: total }, (_, index) => (
          <span
            key={index}
            className={`block rounded-[1px] ${size === "md" ? "h-4 w-1.5" : "h-3 w-1"} ${
              index < present ? "bg-accent" : "bg-surface-3"
            }`}
          />
        ))}
      </span>
      <span className="font-mono text-2xs text-ink-3" data-numeric>
        {present}/{total}
      </span>
    </span>
  );
}
