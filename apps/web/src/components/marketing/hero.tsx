import { ButtonLink } from "@/components/ui/button";
import { DossierPreview } from "./dossier-preview";

export function Hero() {
  return (
    <section className="relative overflow-hidden">
      {/* Halo et grille : repère spatial discret, jamais au premier plan */}
      <div
        aria-hidden
        className="glow-accent pointer-events-none absolute inset-x-0 top-0 h-[520px] opacity-60"
      />
      <div
        aria-hidden
        className="bg-grid mask-fade-b pointer-events-none absolute inset-x-0 top-0 h-[560px] opacity-70"
      />

      <div className="relative mx-auto max-w-6xl px-6 pt-20 pb-16 sm:pt-28">
        <div className="mx-auto max-w-2xl text-center">
          <span className="inline-flex items-center gap-2 rounded-full border border-line bg-surface-1/80 px-3 py-1 text-2xs text-ink-2">
            <span className="size-1.5 rounded-full bg-accent" />
            CRM · LMS vidéo · signature électronique · exports Qualiopi
          </span>

          <h1 className="mt-6 text-4xl leading-[1.05] font-semibold tracking-[-0.04em] sm:text-6xl">
            Le CRM des organismes
            <br />
            de formation.
          </h1>

          <p className="mx-auto mt-5 max-w-xl text-base text-ink-2">
            Pilotez vos apprenants, vos formateurs, votre catalogue et vos sessions
            depuis un seul outil. Les modules vidéo, les questionnaires et les
            signatures y sont intégrés — et la preuve de tout cela se constitue
            pendant que vous travaillez.
          </p>

          <div className="mt-8 flex flex-col items-center justify-center gap-3 sm:flex-row">
            <ButtonLink href="/inscription" size="lg">
              Commencer l&apos;essai
            </ButtonLink>
            <ButtonLink href="#crm" variant="secondary" size="lg">
              Voir le produit
            </ButtonLink>
          </div>

          <p className="mt-4 text-2xs text-ink-3">
            Trente jours, sans carte bancaire · signature électronique intégrée,
            aucun coût par document.
          </p>
        </div>

        <div className="relative mx-auto mt-14 max-w-5xl">
          <DossierPreview />
        </div>
      </div>
    </section>
  );
}
