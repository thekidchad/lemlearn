/**
 * Aperçu statique de l'écran « Fiche dossier » — l'écran signature du produit.
 * Entièrement en HTML/CSS : pas de capture d'écran à re-générer à chaque
 * évolution de l'interface, et le texte reste sélectionnable et lisible par
 * les lecteurs d'écran.
 */

const timeline = [
  { at: "14 janv. · 09:12", label: "Dossier créé", who: "Marie Dubreuil", tone: "ink-3" },
  { at: "14 janv. · 09:40", label: "Devis DEV-2026-0143 envoyé", who: "Resend", tone: "ink-3" },
  { at: "16 janv. · 11:02", label: "Convention signée · OTP e-mail", who: "L. Bertrand", tone: "ok" },
  { at: "16 janv. · 11:02", label: "Scellement PAdES + horodatage TSA", who: "SHA-256 vérifié", tone: "ok" },
  { at: "03 févr. · 18:47", label: "Module 2 terminé · 41 min suivies", who: "couverture 96 %", tone: "ok" },
  { at: "03 févr. · 19:05", label: "Questionnaire post-module · 17/20", who: "2ᵉ tentative", tone: "ok" },
  { at: "04 févr. · 09:00", label: "Émargement en attente", who: "créneau du matin", tone: "warn" },
];

const toneClass: Record<string, string> = {
  ok: "bg-ok",
  warn: "bg-warn",
  "ink-3": "bg-ink-3",
};

const proofs = [
  { label: "Convention signée", state: "ok" },
  { label: "Programme remis", state: "ok" },
  { label: "Relevés de connexion", state: "ok" },
  { label: "Évaluation d'entrée", state: "ok" },
  { label: "Émargements", state: "warn" },
  { label: "Satisfaction à froid", state: "todo" },
];

export function DossierPreview() {
  return (
    <div className="surface-card overflow-hidden p-0 text-left">
      {/* Barre de fenêtre */}
      <div className="flex h-9 items-center gap-2 border-b border-line bg-surface-2/60 px-4">
        <span className="size-2 rounded-full bg-line-strong" />
        <span className="size-2 rounded-full bg-line-strong" />
        <span className="size-2 rounded-full bg-line-strong" />
        <span className="ml-3 font-mono text-2xs text-ink-3">
          app.lemlearn.fr/dossiers/DOS-2026-0143
        </span>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-[168px_1fr]">
        {/* Navigation */}
        <aside className="hidden flex-col gap-0.5 border-r border-line p-3 sm:flex">
          {[
            ["Tableau de bord", false],
            ["Pipeline", false],
            ["Dossiers", true],
            ["Sessions", false],
            ["Catalogue", false],
            ["Questionnaires", false],
            ["Preuves", false],
          ].map(([label, active]) => (
            <span
              key={label as string}
              className={`rounded-md px-2.5 py-1.5 text-xs ${
                active ? "bg-surface-2 text-ink" : "text-ink-3"
              }`}
            >
              {label}
            </span>
          ))}
          <div className="mt-auto hidden rounded-md border border-line px-2.5 py-1.5 font-mono text-2xs text-ink-3 lg:block">
            ⌘K
          </div>
        </aside>

        {/* Contenu */}
        <div className="min-w-0 p-5">
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div>
              <p className="font-mono text-2xs tracking-wide text-ink-3">DOS-2026-0143</p>
              <h3 className="mt-1 text-lg font-semibold tracking-[-0.03em]">
                Léa Bertrand
              </h3>
              <p className="mt-1 text-xs text-ink-2">
                Sécurité incendie — SSIAP 1 · financé par l&apos;OPCO EP
              </p>
            </div>

            {/* Le bordereau des pièces, tel qu'il apparaît dans le produit :
                une case par pièce attendue, parce qu'aucune n'est à moitié
                présente. */}
            <div>
              <p className="eyebrow mb-1.5">Bordereau des pièces</p>
              <span className="flex items-center gap-1.5">
                <span className="flex gap-1" aria-hidden>
                  {Array.from({ length: 13 }, (_, index) => (
                    <span
                      key={index}
                      className={`block h-4 w-1.5 rounded-[1px] ${
                        index < 11 ? "bg-accent" : "bg-surface-3"
                      }`}
                    />
                  ))}
                </span>
                <span className="font-mono text-2xs text-ink-3" data-numeric>
                  11/13
                </span>
              </span>
              <p className="mt-1.5 text-2xs text-ink-3">
                Manque : satisfaction à froid, attestation
              </p>
            </div>
          </div>

          <div className="mt-5 grid gap-5 lg:grid-cols-[1fr_180px]">
            {/* Timeline horodatée */}
            <div>
              <p className="mb-2.5 text-2xs font-medium tracking-wide text-ink-3 uppercase">
                Journal horodaté
              </p>
              <ol className="relative space-y-2.5 border-l border-line pl-4">
                {timeline.map((event) => (
                  <li key={event.label} className="relative">
                    <span
                      className={`absolute top-1.5 -left-[1.3125rem] size-1.5 rounded-full ring-4 ring-surface-1 ${
                        toneClass[event.tone]
                      }`}
                    />
                    <p className="text-xs text-ink">{event.label}</p>
                    <p className="font-mono text-2xs text-ink-3">
                      {event.at} · {event.who}
                    </p>
                  </li>
                ))}
              </ol>
            </div>

            {/* Checklist de preuve */}
            <div className="hidden lg:block">
              <p className="mb-2.5 text-2xs font-medium tracking-wide text-ink-3 uppercase">
                Pièces
              </p>
              <ul className="space-y-1.5">
                {proofs.map((proof) => (
                  <li
                    key={proof.label}
                    className="flex items-center gap-2 text-2xs text-ink-2"
                  >
                    <span
                      className={`size-1.5 shrink-0 rounded-full ${
                        proof.state === "ok"
                          ? "bg-ok"
                          : proof.state === "warn"
                            ? "bg-warn"
                            : "bg-ink-3/50"
                      }`}
                    />
                    {proof.label}
                  </li>
                ))}
              </ul>
              <div className="mt-4 rounded-lg border border-line bg-surface-2/60 p-2.5">
                <p className="text-2xs text-ink-2">Exporter le dossier</p>
                <p className="mt-0.5 font-mono text-2xs text-ink-3">
                  ZIP · 12 pièces · manifest SHA-256
                </p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
