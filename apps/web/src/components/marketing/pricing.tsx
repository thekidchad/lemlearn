import { ButtonLink } from "@/components/ui/button";

const plans = [
  {
    name: "Indépendant",
    price: "49",
    tagline: "Formateur seul, quelques sessions par mois.",
    features: [
      "1 formateur, 50 apprenants actifs",
      "20 h de vidéo hébergée",
      "Signatures illimitées",
      "Exports Qualiopi",
    ],
    cta: "Commencer",
    href: "/inscription?plan=solo",
    highlight: false,
  },
  {
    name: "Organisme",
    price: "149",
    tagline: "L'offre pensée pour un OF certifié Qualiopi.",
    features: [
      "10 formateurs, apprenants illimités",
      "200 h de vidéo hébergée",
      "Questionnaires & analyse par question",
      "Émargement présentiel et distanciel",
      "Archivage inviolable 10 ans",
    ],
    cta: "Demander une démo",
    href: "/demo",
    highlight: true,
  },
  {
    name: "Réseau",
    price: "Sur mesure",
    tagline: "Plusieurs entités, marque blanche, API.",
    features: [
      "Organisations multiples",
      "SSO et rôles avancés",
      "API et webhooks",
      "Accompagnement à la certification",
    ],
    cta: "Nous contacter",
    href: "/contact",
    highlight: false,
  },
];

export function Pricing() {
  return (
    <section id="tarifs" className="py-20">
      <div className="mx-auto max-w-6xl px-6">
        <div className="max-w-2xl">
          <p className="text-2xs font-medium tracking-widest text-accent-ink uppercase">
            Tarifs
          </p>
          <h2 className="mt-3 text-3xl font-semibold tracking-[-0.035em] sm:text-4xl">
            Un abonnement. Pas de facturation à la signature.
          </h2>
          <p className="mt-4 text-ink-2">
            Les prestataires de signature facturent 0,50 € à 1,50 € par document. Ici la
            signature est interne : elle est comprise, quel que soit le volume.
          </p>
        </div>

        <div className="mt-12 grid gap-4 lg:grid-cols-3">
          {plans.map((plan) => (
            <article
              key={plan.name}
              className={`surface-card flex flex-col p-6 ${
                plan.highlight ? "ring-1 ring-accent/40" : ""
              }`}
            >
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-medium">{plan.name}</h3>
                {plan.highlight && (
                  <span className="rounded-md bg-accent-dim px-2 py-0.5 text-2xs text-accent-ink">
                    Le plus choisi
                  </span>
                )}
              </div>

              <p className="mt-4 flex items-baseline gap-1.5">
                <span className="text-3xl font-semibold tracking-[-0.04em]" data-numeric>
                  {plan.price === "Sur mesure" ? plan.price : `${plan.price} €`}
                </span>
                {plan.price !== "Sur mesure" && (
                  <span className="text-xs text-ink-3">/ mois HT</span>
                )}
              </p>
              <p className="mt-2 text-xs text-ink-2">{plan.tagline}</p>

              <ul className="mt-6 flex-1 space-y-2.5">
                {plan.features.map((feature) => (
                  <li key={feature} className="flex gap-2.5 text-xs text-ink-2">
                    <span className="mt-1.5 size-1 shrink-0 rounded-full bg-ok" />
                    {feature}
                  </li>
                ))}
              </ul>

              <ButtonLink
                href={plan.href}
                variant={plan.highlight ? "primary" : "secondary"}
                className="mt-6 w-full"
              >
                {plan.cta}
              </ButtonLink>
            </article>
          ))}
        </div>

        <p className="mt-6 text-center text-2xs text-ink-3">
          Hébergement en France · données chiffrées · réversibilité garantie par export
          complet.
        </p>
      </div>
    </section>
  );
}
