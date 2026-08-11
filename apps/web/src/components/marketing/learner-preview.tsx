import Image from "next/image";

/**
 * Aperçu de l'espace apprenant : lecteur vidéo avec piste de couverture réelle
 * et questionnaire post-module. C'est le couplage assiduité × acquis, montré
 * plutôt que raconté.
 *
 * La piste de couverture distingue ce qui a été *réellement* vu (accent) de ce
 * qui ne l'a pas été (gris) — rejouer trois fois la même minute n'allume pas
 * trois segments.
 */

// 40 segments de 10 s. `true` = segment couvert par au moins un visionnage.
const coverage = Array.from({ length: 40 }, (_, i) => i < 27 || (i > 29 && i < 36));

const answers = [
  { label: "Déclencher l'alarme générale", state: "correct" },
  { label: "Prévenir les secours extérieurs", state: "picked-wrong" },
  { label: "Évacuer la zone sinistrée", state: "idle" },
];

export function LearnerPreview() {
  return (
    <section className="py-20">
      <div className="mx-auto max-w-6xl px-6">
        <div className="grid items-center gap-12 lg:grid-cols-[1fr_1.15fr]">
          <div>
            <p className="text-2xs font-medium tracking-widest text-accent-ink uppercase">
              Espace apprenant
            </p>
            <h2 className="mt-3 text-3xl font-semibold tracking-[-0.035em] sm:text-4xl">
              L&apos;assiduité mesurée, pas déclarée.
            </h2>
            <p className="mt-4 text-ink-2">
              Le lecteur envoie un signal toutes les cinq secondes. Le serveur
              reconstruit la couverture réelle du module et refuse les progressions
              impossibles : accélération, lecture en arrière-plan, sauts.
            </p>
            <p className="mt-4 text-ink-2">
              À la fin de la vidéo, le questionnaire se déplie sans changer de page.
              Chaque réponse est conservée avec son temps de réflexion et le nombre de
              fois où l&apos;apprenant en a changé.
            </p>

            <dl className="mt-8 grid grid-cols-3 gap-px overflow-hidden rounded-xl border border-line bg-line">
              {[
                ["96 %", "couverture"],
                ["41 min", "temps réel"],
                ["17/20", "quiz"],
              ].map(([value, label]) => (
                <div key={label} className="bg-surface-1 px-4 py-3">
                  <dt className="text-2xs text-ink-3">{label}</dt>
                  <dd className="mt-0.5 text-lg font-semibold tracking-[-0.03em]" data-numeric>
                    {value}
                  </dd>
                </div>
              ))}
            </dl>
          </div>

          <div className="surface-card overflow-hidden p-0">
            {/* Cadre vidéo */}
            <div className="relative aspect-video bg-surface-2">
              <Image
                src="/photos/apprenant.jpg"
                alt="Un apprenant casque sur les oreilles suit un module vidéo sur son ordinateur portable."
                fill
                sizes="(max-width: 1024px) 100vw, 620px"
                className="object-cover opacity-70"
              />
              <div
                aria-hidden
                className="absolute inset-0 bg-gradient-to-t from-surface-0/90 via-surface-0/30 to-surface-0/50"
              />
              <div className="absolute inset-0 flex items-center justify-center">
                <span className="flex size-14 items-center justify-center rounded-full border border-line-strong bg-surface-0/70 backdrop-blur">
                  <svg viewBox="0 0 16 16" className="ml-0.5 size-5 text-ink" fill="currentColor">
                    <path d="M4.5 2.6 13 7.6a.5.5 0 0 1 0 .86L4.5 13.4a.5.5 0 0 1-.75-.43V3.03a.5.5 0 0 1 .75-.43Z" />
                  </svg>
                </span>
              </div>
              <p className="absolute top-3 left-4 text-2xs text-ink-2">
                Module 2 — Le SSI et ses composants
              </p>
              <p className="absolute top-3 right-4 font-mono text-2xs text-ink-3">
                18:42 / 19:30
              </p>
            </div>

            {/* Piste de lecture + piste de couverture réelle */}
            <div className="space-y-2 border-t border-line px-4 pt-3 pb-4">
              <div className="h-1 rounded-full bg-surface-3">
                <div className="h-full w-[95%] rounded-full bg-ink-2" />
              </div>
              <div className="flex gap-px" aria-hidden>
                {coverage.map((seen, i) => (
                  <span
                    key={i}
                    className={`h-1.5 flex-1 rounded-[1px] ${seen ? "bg-accent" : "bg-surface-3"}`}
                  />
                ))}
              </div>
              <p className="flex items-center justify-between text-2xs text-ink-3">
                <span>Couverture réelle du module</span>
                <span data-numeric>33 segments sur 40</span>
              </p>
            </div>

            {/* Questionnaire déplié en fin de vidéo */}
            <div className="border-t border-line bg-surface-0/40 p-4">
              <div className="flex items-center justify-between">
                <p className="text-2xs tracking-wide text-ink-3 uppercase">
                  Questionnaire — question 3 sur 8
                </p>
                <span className="font-mono text-2xs text-ink-3">tentative 2</span>
              </div>
              <p className="mt-2.5 text-sm">
                Quel est le premier geste du SSIAP 1 à la réception d&apos;une alarme
                feu&nbsp;?
              </p>
              <ul className="mt-3 space-y-1.5">
                {answers.map((answer) => (
                  <li
                    key={answer.label}
                    className={`flex items-center gap-2.5 rounded-lg border px-3 py-2 text-xs ${
                      answer.state === "correct"
                        ? "border-ok/40 bg-ok/10 text-ink"
                        : answer.state === "picked-wrong"
                          ? "border-bad/40 bg-bad/10 text-ink-2"
                          : "border-line text-ink-2"
                    }`}
                  >
                    <span
                      className={`size-1.5 shrink-0 rounded-full ${
                        answer.state === "correct"
                          ? "bg-ok"
                          : answer.state === "picked-wrong"
                            ? "bg-bad"
                            : "bg-ink-3/50"
                      }`}
                    />
                    {answer.label}
                  </li>
                ))}
              </ul>
              <p className="mt-3 font-mono text-2xs text-ink-3">
                réponse enregistrée · 14 s · 1 changement · 03 févr. 19:05:22
              </p>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
