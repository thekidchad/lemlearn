import type { Metadata } from "next";
import Link from "next/link";
import { apiFetch } from "@/lib/api";

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
  enrollment: { sessionId: string; progress?: Progress[]; finalPassed: boolean };
  session: { id: string; title: string; startsAt: string; endsAt: string };
  course: { id: string; title: string; durationHours: number };
  modules: Module[];
  percent: number;
}

export default async function LearnerPage({ searchParams }: PageProps<"/apprenant">) {
  const params = await searchParams;
  const contactId = typeof params.contactId === "string" ? params.contactId : undefined;

  const { enrollments } = await apiFetch<{ enrollments: Entry[] | null }>(
    `/v1/learn${contactId ? `?contactId=${encodeURIComponent(contactId)}` : ""}`,
  );
  const rows = enrollments ?? [];

  return (
    <>
      <header className="flex h-14 items-center border-b border-line px-6">
        <h1 className="text-sm font-medium">Mon parcours</h1>
      </header>

      {rows.length === 0 ? (
        <p className="px-6 py-16 text-center text-xs text-ink-3">
          Aucune formation en cours.
        </p>
      ) : (
        <div className="space-y-6 p-6">
          {rows.map((entry) => (
            <section key={entry.session.id} className="surface-card p-5">
              <div className="flex flex-wrap items-start justify-between gap-4">
                <div>
                  <h2 className="text-base font-medium">{entry.course.title}</h2>
                  <p className="mt-1 text-xs text-ink-2">
                    {entry.session.title} · {entry.course.durationHours} h
                  </p>
                </div>
                <div className="w-40">
                  <div className="flex items-baseline justify-between">
                    <span className="text-2xs text-ink-3">Progression</span>
                    <span className="text-sm font-semibold text-ok" data-numeric>
                      {entry.percent} %
                    </span>
                  </div>
                  <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-surface-3">
                    <div
                      className="h-full rounded-full bg-ok"
                      style={{ width: `${entry.percent}%` }}
                    />
                  </div>
                </div>
              </div>

              <ol className="mt-5 divide-y divide-line border-t border-line">
                {entry.modules.map((module) => {
                  const progress = entry.enrollment.progress?.find(
                    (p) => p.moduleId === module.id,
                  );
                  const done = Boolean(progress?.completedAt);

                  return (
                    <li key={module.id}>
                      <Link
                        href={`/apprenant/${entry.session.id}/${entry.course.id}/${module.id}${
                          contactId ? `?contactId=${contactId}` : ""
                        }`}
                        className="flex items-center gap-4 py-3 transition-colors duration-[120ms] hover:bg-surface-2/60"
                      >
                        <span
                          className={`flex size-6 shrink-0 items-center justify-center rounded-full border text-2xs ${
                            done
                              ? "border-ok/40 bg-ok/15 text-ok"
                              : "border-line text-ink-3"
                          }`}
                        >
                          {done ? "✓" : module.position}
                        </span>

                        <span className="min-w-0 flex-1">
                          <span className="block truncate text-sm">{module.title}</span>
                          <span className="block text-2xs text-ink-3">
                            {formatDuration(module.durationMs)}
                            {module.quizId ? " · questionnaire" : ""}
                          </span>
                        </span>

                        {/* Les deux conditions de validation sont montrées
                            séparément : un apprenant doit comprendre ce qui
                            lui manque, pas seulement qu'il lui manque quelque
                            chose. */}
                        <span className="hidden items-center gap-3 sm:flex">
                          <Gauge
                            label="vu"
                            value={progress?.coveragePercent ?? 0}
                            target={module.minCoveragePercent}
                          />
                          {module.quizId && (
                            <Gauge
                              label="quiz"
                              value={progress?.quizPercent ?? 0}
                              target={progress?.quizPassed ? 0 : 100}
                              ok={progress?.quizPassed}
                            />
                          )}
                        </span>
                      </Link>
                    </li>
                  );
                })}
              </ol>
            </section>
          ))}
        </div>
      )}
    </>
  );
}

function Gauge({
  label,
  value,
  target,
  ok,
}: {
  label: string;
  value: number;
  target: number;
  ok?: boolean;
}) {
  const reached = ok ?? value >= target;
  return (
    <span className="flex w-20 flex-col gap-1">
      <span className="flex justify-between text-2xs text-ink-3">
        <span>{label}</span>
        <span data-numeric>{value} %</span>
      </span>
      <span className="h-1 overflow-hidden rounded-full bg-surface-3">
        <span
          className={`block h-full rounded-full ${reached ? "bg-ok" : "bg-warn"}`}
          style={{ width: `${Math.min(value, 100)}%` }}
        />
      </span>
    </span>
  );
}

function formatDuration(ms: number): string {
  if (!ms) return "—";
  const minutes = Math.round(ms / 60000);
  return `${minutes} min`;
}
