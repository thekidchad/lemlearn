const pillars = [
  {
    tag: "Qualiopi",
    title: "Traçabilité, assiduité, acquis",
    items: [
      "Relevés de connexion détaillés, exportables par apprenant et par session",
      "Évaluations d'entrée et de sortie horodatées, versions figées",
      "Satisfaction à chaud et à froid programmée automatiquement à M+3",
      "Un module n'est validé que si l'assiduité et le quiz le sont",
    ],
  },
  {
    tag: "RGPD",
    title: "Consentement, effacement, conservation",
    items: [
      "Consentement recueilli, versionné et horodaté",
      "Pièces d'identité chiffrées par clé dédiée, jamais servies en direct",
      "Anonymisation sur demande, sans détruire les pièces à valeur probante",
      "Durées de conservation appliquées automatiquement",
    ],
  },
  {
    tag: "eIDAS",
    title: "Valeur probante des signatures",
    items: [
      "Authentification forte par OTP e-mail ou SMS",
      "Horodatage par autorité qualifiée RFC 3161",
      "Scellement PAdES : un octet modifié invalide la signature",
      "Dossier de preuve : IP, appareil, tracé, chaîne d'audit",
    ],
  },
];

export function Compliance() {
  return (
    <section id="conformite" className="border-y border-line bg-surface-1/40 py-20">
      <div className="mx-auto max-w-6xl px-6">
        <div className="max-w-2xl">
          <p className="text-2xs font-medium tracking-widest text-accent-ink uppercase">
            Conformité
          </p>
          <h2 className="mt-3 text-3xl font-semibold tracking-[-0.035em] sm:text-4xl">
            Ce n&apos;est pas une case à cocher. C&apos;est l&apos;architecture.
          </h2>
          <p className="mt-4 text-ink-2">
            L&apos;horodatage, le scellement et le journal d&apos;audit ne sont pas des
            options activables : ils sont dans le chemin d&apos;écriture de chaque
            opération.
          </p>
        </div>

        <div className="mt-12 grid gap-4 lg:grid-cols-3">
          {pillars.map((pillar) => (
            <article key={pillar.tag} className="surface-card p-5">
              <div className="flex items-center gap-2">
                <span className="rounded-md bg-accent-dim px-2 py-0.5 font-mono text-2xs tracking-wide text-accent-ink">
                  {pillar.tag}
                </span>
              </div>
              <h3 className="mt-3 text-sm font-medium">{pillar.title}</h3>
              <ul className="mt-4 space-y-2.5">
                {pillar.items.map((item) => (
                  <li key={item} className="flex gap-2.5 text-xs leading-relaxed text-ink-2">
                    <span className="mt-1.5 size-1 shrink-0 rounded-full bg-ok" />
                    {item}
                  </li>
                ))}
              </ul>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}
