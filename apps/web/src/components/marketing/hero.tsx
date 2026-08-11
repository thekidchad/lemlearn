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
            <span className="size-1.5 rounded-full bg-ok" />
            Qualiopi · eIDAS · RGPD — la preuve est produite par l&apos;outil
          </span>

          <h1 className="mt-6 text-4xl leading-[1.05] font-semibold tracking-[-0.04em] sm:text-6xl">
            Votre audit Qualiopi
            <br />
            est déjà prêt.
          </h1>

          <p className="mx-auto mt-5 max-w-xl text-base text-ink-2">
            lemlearn réunit le CRM commercial, le LMS vidéo et la chaîne de preuve
            horodatée. Chaque minute suivie, chaque réponse de questionnaire, chaque
            signature est enregistrée, scellée et exportable en un clic.
          </p>

          <div className="mt-8 flex flex-col items-center justify-center gap-3 sm:flex-row">
            <ButtonLink href="/demo" size="lg">
              Demander une démo
            </ButtonLink>
            <ButtonLink href="#preuve" variant="secondary" size="lg">
              Voir la chaîne de preuve
            </ButtonLink>
          </div>

          <p className="mt-4 text-2xs text-ink-3">
            Signature électronique intégrée — aucun coût par document.
          </p>
        </div>

        <div className="relative mx-auto mt-14 max-w-5xl">
          <DossierPreview />
        </div>
      </div>
    </section>
  );
}
