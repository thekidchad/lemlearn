import Image from "next/image";

const audiences = [
  {
    title: "Organismes certifiés",
    body: "Vous passez un audit tous les dix-huit mois et vous perdez des jours à rassembler les preuves.",
  },
  {
    title: "Formateurs indépendants",
    body: "Vous jonglez entre un tableur, une boîte mail et un service de signature facturé au document.",
  },
  {
    title: "Réseaux et franchises",
    body: "Vous pilotez plusieurs entités et vous voulez un standard de preuve identique partout.",
  },
];

export function Audience() {
  return (
    <section className="border-y border-line bg-surface-1/40 py-20">
      <div className="mx-auto max-w-6xl px-6">
        <div className="grid items-center gap-12 lg:grid-cols-[1.1fr_1fr]">
          <div className="relative aspect-[16/10] overflow-hidden rounded-xl border border-line">
            <Image
              src="/photos/formatrice.jpg"
              alt="Une formatrice debout explique à trois stagiaires assis autour d'une table dans une salle de formation lumineuse."
              fill
              sizes="(max-width: 1024px) 100vw, 620px"
              className="object-cover"
            />
            <div
              aria-hidden
              className="absolute inset-0 bg-gradient-to-tr from-surface-0/60 via-transparent to-transparent"
            />
          </div>

          <div>
            <p className="text-2xs font-medium tracking-widest text-accent-ink uppercase">
              Pour qui
            </p>
            <h2 className="mt-3 text-3xl font-semibold tracking-[-0.035em] sm:text-4xl">
              Vous formez. L&apos;outil documente.
            </h2>
            <p className="mt-4 text-ink-2">
              La charge administrative d&apos;un organisme de formation ne vient pas de
              la pédagogie, mais de sa justification. C&apos;est cette moitié-là que
              lemlearn prend en charge.
            </p>

            <dl className="mt-8 divide-y divide-line border-y border-line">
              {audiences.map((audience) => (
                <div key={audience.title} className="py-4">
                  <dt className="text-sm font-medium">{audience.title}</dt>
                  <dd className="mt-1 text-xs leading-relaxed text-ink-2">
                    {audience.body}
                  </dd>
                </div>
              ))}
            </dl>
          </div>
        </div>
      </div>
    </section>
  );
}
