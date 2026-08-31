import type { Metadata } from "next";
import Link from "next/link";
import { CourseCover } from "@/components/app/course-cover";
import { apiFetch, ApiError, contactName, type Contact, type Me } from "@/lib/api";

export const metadata: Metadata = { title: "Mon parcours" };

interface Module {
  id: string;
  title: string;
  position: number;
  durationMs: number;
  quizId?: string;
  minCoveragePercent: number;
}

interface Progress {
  moduleId: string;
  coveragePercent: number;
  quizPassed: boolean;
  quizPercent: number;
  completedAt?: string;
}

interface Entry {
  enrollment: { sessionId: string; progress?: Progress[] | null; finalPassed: boolean };
  session: { id: string; title: string; startsAt: string; endsAt: string };
  course: { id: string; title: string; durationHours: number };
  modules: Module[] | null;
  percent: number;
  coverUrl?: string;
  done: number;
}

/**
 * L'espace apprenant.
 *
 * Il ne ressemble pas au reste du produit, et c'est délibéré. Le CRM est dense
 * parce qu'une assistante de formation y compare des lignes toute la journée ;
 * l'apprenant ouvre le sien le soir, souvent sur un téléphone, pour une seule
 * question : où j'en étais. L'écran répond à celle-là d'abord — une formation,
 * une action — et range le reste en dessous.
 */
export default async function LearnerPage({ searchParams }: PageProps<"/apprenant">) {
  const params = await searchParams;
  const contactId = typeof params.contactId === "string" ? params.contactId : undefined;

  // Un administrateur n'a pas de fiche apprenant : l'API le dit, et sans ce
  // rattrapage l'écran affichait une erreur au lieu de proposer de choisir un
  // apprenant. C'est la même page, vue depuis l'autre côté du bureau.
  let rows: Entry[];
  try {
    const { enrollments } = await apiFetch<{ enrollments: Entry[] | null }>(
      `/v1/learn${contactId ? `?contactId=${encodeURIComponent(contactId)}` : ""}`,
    );
    rows = enrollments ?? [];
  } catch (error) {
    if (!(error instanceof ApiError) || error.status !== 403) throw error;

    const { contacts } = await apiFetch<{ contacts: Contact[] | null }>(
      "/v1/contacts?kind=learner",
    ).catch(() => ({ contacts: null }));
    return <LearnerPicker learners={contacts ?? []} />;
  }

  const me = await apiFetch<Me>("/v1/me");
  const suffix = contactId ? `?contactId=${encodeURIComponent(contactId)}` : "";

  return (
    <div className="mx-auto max-w-3xl px-5 py-12 sm:px-8 sm:py-16">
      <h1 className="learner-title">{greeting(me.user.firstName)}</h1>
      <p className="learner-body mt-3">
        {rows.length === 0
          ? "Aucune formation ne vous est encore ouverte."
          : "Reprenez là où vous en étiez."}
      </p>

      {rows.length === 0 ? (
        <p className="mt-10 rounded-xl border border-line p-6 text-sm text-ink-2">
          Votre organisme vous inscrira à une session : elle apparaîtra ici, avec
          vos modules et votre attestation.
        </p>
      ) : (
        <div className="mt-10 space-y-8">
          {rows.map((entry) => (
            <CourseCard key={entry.session.id} entry={entry} suffix={suffix} />
          ))}
        </div>
      )}
    </div>
  );
}

/**
 * Une formation, et une seule action évidente.
 *
 * Le module à reprendre est mis en avant seul ; la liste complète se déplie en
 * dessous. Un espace qui déballe d'emblée tous les modules oblige à chercher le
 * sien à chaque visite — c'est le réflexe du tableau de bord, et ce n'est pas
 * ce dont on a besoin ici.
 */
function CourseCard({ entry, suffix }: { entry: Entry; suffix: string }) {
  const modules = entry.modules ?? [];
  const done = (module: Module) =>
    Boolean(entry.enrollment.progress?.find((p) => p.moduleId === module.id)?.completedAt);

  const next = modules.find((module) => !done(module));

  return (
    <section className="surface-card overflow-hidden">
      <div className="p-5 sm:p-6">
        <CourseCover title={entry.course.title} url={entry.coverUrl} />

        <h2 className="learner-heading mt-5">{entry.course.title}</h2>
        <p className="mt-1 text-sm text-ink-3">
          {entry.session.title} · {entry.course.durationHours} h
        </p>

        {/* La progression se compte en modules. Aucun module n'est à moitié
            suivi, et une jauge continue laisserait croire le contraire — c'est
            la même règle que le bordereau des pièces d'un dossier. */}
        {modules.length > 0 && (
          <div className="mt-5">
            <div className="flex gap-1" aria-hidden>
              {modules.map((module) => (
                <span key={module.id} className="learner-mark" data-done={done(module)} />
              ))}
            </div>
            <p className="mt-2 text-sm text-ink-2" data-numeric>
              {entry.done} module{entry.done > 1 ? "s" : ""} sur {modules.length}
              {modules.length > 0 && !next ? " — parcours terminé" : ""}
            </p>
          </div>
        )}

        {next ? (
          <Link
            href={`/apprenant/${entry.session.id}/${next.position}${suffix}`}
            className="mt-6 flex items-center gap-4 rounded-xl bg-accent px-5 py-4 text-accent-ink transition-opacity duration-[120ms] hover:opacity-90"
          >
            <span className="min-w-0 flex-1">
              <span className="block text-xs opacity-80">
                {entry.done === 0 ? "Commencer" : "Reprendre"} · module {next.position}
              </span>
              <span className="mt-0.5 block truncate text-base font-medium">{next.title}</span>
            </span>
            <span aria-hidden className="text-lg">
              →
            </span>
          </Link>
        ) : modules.length > 0 ? (
          <p className="mt-6 rounded-xl border border-ok/40 bg-ok/10 px-5 py-4 text-sm text-ok">
            Tous vos modules sont terminés.
          </p>
        ) : (
          <p className="mt-6 text-sm text-ink-3">
            Cette formation n&apos;a pas encore de module.
          </p>
        )}

        <Link
          href={`/apprenant/${entry.session.id}/emargement${suffix}`}
          className="mt-4 inline-block text-sm text-ink-2 underline underline-offset-4 hover:text-ink"
        >
          Confirmer ma présence
        </Link>
      </div>

      {modules.length > 1 && (
        <details className="border-t border-line">
          <summary className="cursor-pointer px-5 py-3.5 text-sm text-ink-2 hover:text-ink sm:px-6">
            Tous les modules
          </summary>
          <ol className="border-t border-line">
            {modules.map((module) => (
              <li key={module.id}>
                <Link
                  href={`/apprenant/${entry.session.id}/${module.position}${suffix}`}
                  className="flex items-center gap-4 px-5 py-3.5 transition-colors duration-[120ms] hover:bg-surface-2/60 sm:px-6"
                >
                  <span
                    className={`flex size-7 shrink-0 items-center justify-center rounded-full border text-xs ${
                      done(module) ? "border-ok/40 bg-ok/15 text-ok" : "border-line text-ink-3"
                    }`}
                  >
                    {done(module) ? "✓" : module.position}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm">{module.title}</span>
                    <span className="block text-2xs text-ink-3">
                      {formatDuration(module.durationMs)}
                      {module.quizId ? " · questionnaire" : ""}
                    </span>
                  </span>
                </Link>
              </li>
            ))}
          </ol>
        </details>
      )}
    </section>
  );
}

/** Le prénom quand on l'a. Sinon on salue sans nommer, plutôt que « Bonjour , ». */
function greeting(firstName?: string): string {
  const prenom = (firstName ?? "").trim();
  return prenom ? `Bonjour ${prenom}.` : "Bonjour.";
}

function formatDuration(ms: number): string {
  if (!ms) return "—";
  const minutes = Math.round(ms / 60000);
  return `${minutes} min`;
}

/**
 * Choix de l'apprenant, pour un compte qui n'en est pas un.
 *
 * L'espace apprenant se consulte alors « à la place de » : c'est ce que fait
 * une assistante de formation au téléphone quand un stagiaire demande où il en
 * est.
 */
function LearnerPicker({ learners }: { learners: Contact[] }) {
  return (
    <div className="mx-auto max-w-2xl px-5 py-12 sm:px-8">
      <h1 className="learner-heading">Espace apprenant</h1>
      <p className="learner-body mt-3">
        Votre compte gère l&apos;organisme, il n&apos;est rattaché à aucune fiche
        apprenant. Choisissez quelqu&apos;un pour voir son parcours tel qu&apos;il le
        voit.
      </p>

      {learners.length === 0 ? (
        <p className="mt-8 text-sm text-ink-3">
          Aucun apprenant enregistré. Ils se créent dans{" "}
          <Link href="/stagiaires" className="underline hover:text-ink">
            Stagiaires
          </Link>
          .
        </p>
      ) : (
        <div className="mt-8 space-y-px overflow-hidden rounded-xl border border-line bg-line">
          {learners.map((learner) => (
            <Link
              key={learner.id}
              href={`/apprenant?contactId=${learner.id}`}
              className="flex items-center justify-between gap-4 bg-surface-1 px-4 py-3.5 text-sm hover:bg-surface-2"
            >
              <span className="truncate">{contactName(learner)}</span>
              <span className="truncate font-mono text-2xs text-ink-3">
                {learner.email ?? ""}
              </span>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
