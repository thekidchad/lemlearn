import Image from "next/image";

const requirements = [
  {
    title: "Authentification forte",
    body: "Lien unique à usage unique, puis code à 6 chiffres envoyé par e-mail ou SMS. Trois essais, dix minutes de validité.",
  },
  {
    title: "Horodatage opposable",
    body: "L'empreinte du document est horodatée par une autorité RFC 3161. C'est un tiers qui date la signature, pas notre serveur.",
  },
  {
    title: "Dossier de preuve",
    body: "Adresse IP, appareil, e-mail certifié, tracé de la signature, chronologie complète : tout est consigné dans une annexe jointe au document.",
  },
  {
    title: "Intégrité scellée",
    body: "Le PDF est signé au format PAdES. Un seul octet modifié après coup invalide la signature, visiblement, dans n'importe quel lecteur.",
  },
];

export function SignatureSection() {
  return (
    <section className="py-20">
      <div className="mx-auto max-w-6xl px-6">
        <div className="grid items-center gap-12 lg:grid-cols-2">
          <div className="relative aspect-[4/3] overflow-hidden rounded-xl border border-line">
            <Image
              src="/photos/signature.jpg"
              alt="Une main signant du bout d'un stylet sur une tablette posée sur un bureau, à côté d'un document imprimé."
              fill
              sizes="(max-width: 1024px) 100vw, 560px"
              className="object-cover"
            />
            <div
              aria-hidden
              className="absolute inset-0 bg-gradient-to-t from-surface-0/70 via-transparent to-transparent"
            />
            <div className="absolute right-4 bottom-4 left-4 flex items-center justify-between rounded-lg border border-line/60 bg-surface-0/80 px-3 py-2 backdrop-blur">
              <span className="text-2xs text-ink-2">Convention CONV-2026-0143</span>
              <span className="flex items-center gap-1.5 font-mono text-2xs text-ok">
                <span className="size-1.5 rounded-full bg-ok" />
                scellée 16 janv. 11:02:41
              </span>
            </div>
          </div>

          <div>
            <p className="text-2xs font-medium tracking-widest text-accent-ink uppercase">
              Signature électronique
            </p>
            <h2 className="mt-3 text-3xl font-semibold tracking-[-0.035em] sm:text-4xl">
              Signer chez vous, pas chez un tiers.
            </h2>
            <p className="mt-4 text-ink-2">
              Les prestataires facturent chaque document. Nous avons intégré la
              signature à la plateforme, en respectant les quatre exigences qui lui
              donnent sa valeur probante devant un financeur.
            </p>

            <dl className="mt-8 space-y-5">
              {requirements.map((requirement, index) => (
                <div key={requirement.title} className="flex gap-4">
                  <span className="mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-md border border-line bg-surface-2 font-mono text-2xs text-accent-ink">
                    {index + 1}
                  </span>
                  <div>
                    <dt className="text-sm font-medium">{requirement.title}</dt>
                    <dd className="mt-1 text-xs leading-relaxed text-ink-2">
                      {requirement.body}
                    </dd>
                  </div>
                </div>
              ))}
            </dl>
          </div>
        </div>
      </div>
    </section>
  );
}
