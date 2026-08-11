const questions = [
  {
    q: "Votre signature interne est-elle acceptée par les OPCO ?",
    a: "Elle repose sur les quatre exigences qui donnent sa valeur probante à une signature électronique : authentification forte par OTP, horodatage par une autorité qualifiée RFC 3161, dossier de preuve complet (IP, appareil, e-mail certifié, tracé) et scellement cryptographique du PDF. Le document signé se vérifie dans Adobe Reader comme celui d'un prestataire tiers.",
  },
  {
    q: "Comment prouvez-vous l'assiduité sur une formation en vidéo ?",
    a: "Le lecteur envoie un signal toutes les cinq secondes et le serveur reconstitue la couverture réelle du module, segment par segment. Rejouer trois fois la même minute compte pour une minute. Le relevé de connexion exporté détaille chaque séance : date, durée, position, appareil.",
  },
  {
    q: "Que se passe-t-il si un apprenant demande la suppression de ses données ?",
    a: "Ses données personnelles sont anonymisées immédiatement. Les pièces à valeur probante — convention signée, émargements, attestation — restent archivées le temps légal exigé pour un contrôle, mais rattachées à un pseudonyme. C'est la seule lecture compatible à la fois avec le RGPD et avec vos obligations de conservation.",
  },
  {
    q: "Peut-on récupérer ses données si l'on part ?",
    a: "Oui, et c'est la même fonction que celle utilisée pour les audits : un export ZIP complet, dossier par dossier, avec les documents originaux, un manifeste d'empreintes SHA-256 et le journal d'audit. Rien n'est retenu en otage.",
  },
  {
    q: "Faut-il un questionnaire après chaque vidéo ?",
    a: "Ce n'est pas obligatoire, mais c'est fortement recommandé : c'est le couplage entre l'assiduité et l'acquisition des compétences qui rend un dossier solide. Le questionnaire post-module est formatif — le corrigé et l'explication s'affichent après la soumission.",
  },
];

export function Faq() {
  return (
    <section className="border-t border-line py-20">
      <div className="mx-auto grid max-w-6xl gap-10 px-6 lg:grid-cols-[300px_1fr]">
        <div>
          <h2 className="text-3xl font-semibold tracking-[-0.035em]">
            Questions fréquentes
          </h2>
          <p className="mt-4 text-xs leading-relaxed text-ink-2">
            Une question qui n&apos;est pas ici ? Écrivez à{" "}
            <a
              href="mailto:bonjour@lemlearn.fr"
              className="text-accent-ink underline-offset-4 hover:underline"
            >
              bonjour@lemlearn.fr
            </a>
            .
          </p>
        </div>

        <div className="divide-y divide-line border-y border-line">
          {questions.map((item) => (
            <details key={item.q} className="group py-4">
              <summary className="flex cursor-pointer list-none items-start justify-between gap-6 text-sm font-medium">
                {item.q}
                <span
                  aria-hidden
                  className="mt-0.5 shrink-0 text-ink-3 transition-transform duration-[120ms] ease-[var(--ease-out-quint)] group-open:rotate-45"
                >
                  <svg viewBox="0 0 12 12" className="size-3" fill="none">
                    <path
                      d="M6 1v10M1 6h10"
                      stroke="currentColor"
                      strokeWidth="1.5"
                      strokeLinecap="round"
                    />
                  </svg>
                </span>
              </summary>
              <p className="mt-3 max-w-2xl text-xs leading-relaxed text-ink-2">{item.a}</p>
            </details>
          ))}
        </div>
      </div>
    </section>
  );
}
