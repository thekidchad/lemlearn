const steps = [
  {
    title: "Inscription",
    detail: "Fiche apprenant complète, pièce d'identité chiffrée, consentement RGPD horodaté.",
    proof: "Consentement v2.1 · 14 janv. 09:12",
  },
  {
    title: "Convention signée",
    detail: "Lien unique, OTP à 6 chiffres, mention manuscrite guidée, tracé enregistré.",
    proof: "PAdES-B-T · jeton TSA RFC 3161",
  },
  {
    title: "Vidéo suivie",
    detail: "Heartbeat toutes les 5 s, couverture réelle des segments — rejouer ne compte pas double.",
    proof: "41 min · couverture 96 %",
  },
  {
    title: "Questionnaire",
    detail: "Chaque réponse, son temps de réflexion et ses changements sont conservés.",
    proof: "17/20 · version V1 figée",
  },
  {
    title: "Émargement",
    detail: "Feuille de présence par créneau, signée par l'apprenant et contresignée formateur.",
    proof: "Scellé SHA-256 · WORM S3",
  },
  {
    title: "Dossier exporté",
    detail: "ZIP complet avec manifeste d'empreintes et journal d'audit chaîné.",
    proof: "12 pièces · prêt pour l'auditeur",
  },
];

export function ProofChain() {
  return (
    <section id="preuve" className="relative border-y border-line bg-surface-1/40 py-20">
      <div className="mx-auto max-w-6xl px-6">
        <div className="max-w-2xl">
          <p className="text-2xs font-medium tracking-widest text-accent-ink uppercase">
            Chaîne de preuve
          </p>
          <h2 className="mt-3 text-3xl font-semibold tracking-[-0.035em] sm:text-4xl">
            La preuve ne se reconstitue pas. Elle se produit au fil de l&apos;eau.
          </h2>
          <p className="mt-4 text-ink-2">
            Chaque étape du parcours dépose une pièce datée et scellée. Le jour de
            l&apos;audit, il n&apos;y a rien à rassembler : le dossier existe déjà.
          </p>
        </div>

        <ol className="mt-12 grid gap-px overflow-hidden rounded-xl border border-line bg-line md:grid-cols-2 lg:grid-cols-3">
          {steps.map((step, index) => (
            <li key={step.title} className="relative bg-surface-1 p-5">
              <div className="flex items-center gap-2.5">
                <span className="flex size-6 items-center justify-center rounded-md border border-line bg-surface-2 font-mono text-2xs text-accent-ink">
                  {String(index + 1).padStart(2, "0")}
                </span>
                <h3 className="text-sm font-medium">{step.title}</h3>
              </div>
              <p className="mt-3 text-xs leading-relaxed text-ink-2">{step.detail}</p>
              <p className="mt-3 flex items-center gap-1.5 font-mono text-2xs text-ink-3">
                <span className="size-1 rounded-full bg-ok" />
                {step.proof}
              </p>
            </li>
          ))}
        </ol>
      </div>
    </section>
  );
}
