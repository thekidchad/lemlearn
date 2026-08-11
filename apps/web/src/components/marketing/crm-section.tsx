/**
 * Section CRM — la moitié du produit qu'on utilise tous les jours.
 *
 * La chaîne de preuve est le différenciateur, mais ce n'est pas ce qu'on ouvre
 * le lundi matin : on ouvre le pipeline, la fiche d'un apprenant, le planning
 * d'une session. Cette section montre ça.
 */

const columns = [
  {
    stage: "Prospect",
    count: 6,
    tone: "ink-3",
    cards: [
      { name: "Groupe Aramis", detail: "SSIAP 1 · 3 apprenants", amount: "3 750 €", proof: 8 },
      { name: "Mairie de Valence", detail: "Habilitation électrique", amount: "2 400 €", proof: 0 },
    ],
  },
  {
    stage: "Devis",
    count: 4,
    tone: "ink-3",
    cards: [
      { name: "Transports Meunier", detail: "CACES R489 · 8 apprenants", amount: "9 600 €", proof: 23 },
    ],
  },
  {
    stage: "Convention",
    count: 3,
    tone: "warn",
    cards: [
      { name: "Léa Bertrand", detail: "SSIAP 1 · OPCO EP", amount: "1 250 €", proof: 54 },
      { name: "Clinique du Parc", detail: "Gestes et postures", amount: "4 200 €", proof: 46 },
    ],
  },
  {
    stage: "En formation",
    count: 9,
    tone: "ok",
    cards: [
      { name: "Karim Nasri", detail: "SSIAP 1 · module 2/4", amount: "1 250 €", proof: 83 },
    ],
  },
];

const toneDot: Record<string, string> = {
  ok: "bg-ok",
  warn: "bg-warn",
  "ink-3": "bg-ink-3",
};

const capabilities = [
  {
    title: "Apprenants et entreprises",
    body: "Fiche complète — identité, date de naissance, adresse, pièce d'identité chiffrée — entreprises clientes, financeurs OPCO et subrogation de paiement.",
  },
  {
    title: "Formateurs",
    body: "Affectation aux sessions, planning personnel, contresignature des émargements, accès limité à leurs propres groupes.",
  },
  {
    title: "Catalogue et sessions",
    body: "Formations découpées en modules, objectifs, prérequis, tarifs. Sessions présentielles, classes virtuelles ou asynchrones, avec étiquettes libres.",
  },
  {
    title: "Devis et conventions",
    body: "Générés depuis le dossier, sans ressaisie. Le devis accepté devient convention, la convention signée fait basculer le dossier en formation.",
  },
];

export function CrmSection() {
  return (
    <section id="crm" className="py-20">
      <div className="mx-auto max-w-6xl px-6">
        <div className="max-w-2xl">
          <p className="text-2xs font-medium tracking-widest text-accent-ink uppercase">
            Le CRM
          </p>
          <h2 className="mt-3 text-3xl font-semibold tracking-[-0.035em] sm:text-4xl">
            D&apos;abord, un outil pour piloter votre activité.
          </h2>
          <p className="mt-4 text-ink-2">
            Vos apprenants, vos entreprises clientes, vos formateurs, votre catalogue,
            vos sessions et vos devis au même endroit. La preuve, elle, se constitue
            toute seule pendant que vous travaillez.
          </p>
        </div>

        {/* Pipeline */}
        <div className="surface-card mt-12 overflow-hidden p-0">
          <div className="flex items-center justify-between border-b border-line px-4 py-3">
            <div className="flex items-center gap-2">
              <span className="text-sm font-medium">Pipeline</span>
              <span className="rounded-md border border-line bg-surface-2 px-1.5 py-0.5 font-mono text-2xs text-ink-3">
                22 dossiers
              </span>
            </div>
            <div className="hidden items-center gap-1.5 sm:flex">
              {["#présentiel", "#OPCO-validé", "#Q3"].map((tag) => (
                <span
                  key={tag}
                  className="rounded-md border border-line bg-surface-2/60 px-2 py-0.5 text-2xs text-ink-3"
                >
                  {tag}
                </span>
              ))}
            </div>
          </div>

          <div className="grid gap-px bg-line sm:grid-cols-2 lg:grid-cols-4">
            {columns.map((column) => (
              <div key={column.stage} className="min-h-52 bg-surface-1 p-3">
                <div className="flex items-center gap-2 px-1 pb-3">
                  <span className={`size-1.5 rounded-full ${toneDot[column.tone]}`} />
                  <span className="text-xs font-medium">{column.stage}</span>
                  <span className="ml-auto font-mono text-2xs text-ink-3" data-numeric>
                    {column.count}
                  </span>
                </div>

                <div className="space-y-2">
                  {column.cards.map((card) => (
                    <article
                      key={card.name}
                      className="rounded-lg border border-line bg-surface-2/70 p-2.5"
                    >
                      <p className="text-xs font-medium">{card.name}</p>
                      <p className="mt-0.5 text-2xs text-ink-3">{card.detail}</p>
                      <div className="mt-2.5 flex items-center justify-between gap-2">
                        <span className="font-mono text-2xs text-ink-2" data-numeric>
                          {card.amount}
                        </span>
                        {/* Pastille de complétude du dossier de preuve :
                            visible dès le pipeline, pas seulement dans la fiche. */}
                        <span className="flex items-center gap-1.5">
                          <span className="h-1 w-10 overflow-hidden rounded-full bg-surface-3">
                            <span
                              className={`block h-full rounded-full ${
                                card.proof >= 80 ? "bg-ok" : card.proof >= 40 ? "bg-warn" : "bg-ink-3"
                              }`}
                              style={{ width: `${card.proof}%` }}
                            />
                          </span>
                          <span className="font-mono text-2xs text-ink-3" data-numeric>
                            {card.proof}%
                          </span>
                        </span>
                      </div>
                    </article>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>

        <dl className="mt-4 grid gap-4 md:grid-cols-2 lg:grid-cols-4">
          {capabilities.map((capability) => (
            <div key={capability.title} className="surface-card p-4">
              <dt className="text-sm font-medium">{capability.title}</dt>
              <dd className="mt-2 text-xs leading-relaxed text-ink-2">{capability.body}</dd>
            </div>
          ))}
        </dl>
      </div>
    </section>
  );
}
