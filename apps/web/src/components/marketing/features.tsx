const features = [
  {
    title: "CRM de formation",
    body: "Pipeline commercial, fiches apprenants, entreprises clientes et financeurs OPCO. Devis, conventions et contrats générés depuis le dossier, sans ressaisie.",
    points: ["Pipeline glisser-déposer", "Financeurs & subrogation", "Devis en un clic"],
  },
  {
    title: "LMS vidéo intégré",
    body: "Vidéos diffusées en HLS derrière des liens signés à durée courte. Le temps réellement visionné est mesuré segment par segment, pas déclaré.",
    points: ["Reprise de lecture", "Couverture réelle", "Relevé de connexion"],
  },
  {
    title: "Questionnaires à chaque étape",
    body: "Positionnement, quiz après chaque vidéo, évaluation finale, satisfaction à chaud et à froid. Toutes les réponses sont conservées et versionnées.",
    points: ["7 types de questions", "Corrigé & barème", "Analyse par question"],
  },
  {
    title: "Signature électronique interne",
    body: "OTP par e-mail ou SMS, horodatage par autorité RFC 3161, scellement PAdES. Conforme eIDAS, sans facturation au document.",
    points: ["Dossier de preuve", "Vérifiable dans Adobe", "Archivage inviolable"],
  },
  {
    title: "Émargement numérique",
    body: "Feuilles de présence par créneau, en présentiel comme en asynchrone, pré-remplies par les données de visionnage et contresignées par le formateur.",
    points: ["Présentiel & distanciel", "Contresignature", "Horodatage à la minute"],
  },
  {
    title: "Export en un clic",
    body: "Tout le dossier — identité, contrats, relevés, évaluations, émargements, attestation — dans un ZIP avec manifeste d'empreintes et journal d'audit.",
    points: ["Manifeste SHA-256", "Journal chaîné", "Prêt pour l'OPCO"],
  },
];

export function Features() {
  return (
    <section id="produit" className="py-20">
      <div className="mx-auto max-w-6xl px-6">
        <div className="max-w-2xl">
          <p className="text-2xs font-medium tracking-widest text-accent-ink uppercase">
            Récapitulatif
          </p>
          <h2 className="mt-3 text-3xl font-semibold tracking-[-0.035em] sm:text-4xl">
            Un seul outil, du premier appel à l&apos;attestation.
          </h2>
          <p className="mt-4 text-ink-2">
            Plus de CRM d&apos;un côté, de plateforme e-learning de l&apos;autre, et
            d&apos;un Drive au milieu pour recoller les morceaux.
          </p>
        </div>

        <div className="mt-12 grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {features.map((feature) => (
            <article key={feature.title} className="surface-card p-5">
              <h3 className="text-sm font-medium">{feature.title}</h3>
              <p className="mt-2.5 text-xs leading-relaxed text-ink-2">{feature.body}</p>
              <ul className="mt-4 flex flex-wrap gap-1.5">
                {feature.points.map((point) => (
                  <li
                    key={point}
                    className="rounded-md border border-line bg-surface-2/60 px-2 py-0.5 text-2xs text-ink-3"
                  >
                    {point}
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
