import type { Results } from "@/app/(app)/questionnaires/[quizId]/page";

/**
 * Résultats d'un questionnaire, par question puis par apprenant.
 *
 * L'ordre n'est pas neutre : la vue par question est celle qui améliore une
 * formation. Une question que tout le monde rate en dit plus sur le cours que
 * sur les apprenants, et un distracteur que personne ne choisit ne sert à
 * rien.
 */
export function QuizResults({ results }: { results: Results }) {
  const rate = results.submitted > 0 ? (results.passed / results.submitted) * 100 : 0;

  return (
    <section className="surface-card p-5">
      <h2 className="text-sm font-medium">Résultats</h2>

      <dl className="mt-4 grid grid-cols-3 gap-px overflow-hidden rounded-lg border border-line bg-line">
        <Stat label="passations" value={String(results.submitted)} />
        <Stat label="réussite" value={`${Math.round(rate)} %`} />
        <Stat
          label="durée moyenne"
          value={`${Math.round(results.averageDurationMs / 60000)} min`}
        />
      </dl>

      <h3 className="mt-6 text-xs font-medium">Par question</h3>
      <div className="mt-2 space-y-2">
        {results.questions.map((question) => {
          const success = question.answered > 0 ? (question.correct / question.answered) * 100 : 0;
          return (
            <div key={question.questionId} className="rounded-lg border border-line p-3">
              <div className="flex items-start justify-between gap-4">
                <p className="text-xs">{question.prompt || question.questionId}</p>
                <p
                  className={`shrink-0 font-mono text-2xs ${
                    success < 50 ? "text-warn" : "text-ink-3"
                  }`}
                  data-numeric
                >
                  {Math.round(success)} % · {question.answered} rép.
                </p>
              </div>

              <div className="mt-2 h-1 overflow-hidden rounded-full bg-surface-3">
                <div
                  className={`h-full ${success < 50 ? "bg-warn" : "bg-ok"}`}
                  style={{ width: `${success}%` }}
                />
              </div>

              {question.choices && Object.keys(question.choices).length > 0 && (
                <p className="mt-2 font-mono text-2xs text-ink-3">
                  {Object.entries(question.choices)
                    .sort(([, a], [, b]) => b - a)
                    .map(([choice, count]) => `${choice} ×${count}`)
                    .join(" · ")}
                </p>
              )}
            </div>
          );
        })}
      </div>

      <h3 className="mt-6 text-xs font-medium">Par passation</h3>
      <div className="mt-2 overflow-x-auto">
        <table className="w-full text-2xs">
          <thead className="text-ink-3">
            <tr className="text-left">
              <th className="py-1.5 pr-4 font-normal">Inscription</th>
              <th className="py-1.5 pr-4 font-normal">Tentative</th>
              <th className="py-1.5 pr-4 font-normal">Version</th>
              <th className="py-1.5 pr-4 font-normal">Score</th>
              <th className="py-1.5 pr-4 font-normal">Durée</th>
              <th className="py-1.5 font-normal">Passée le</th>
            </tr>
          </thead>
          <tbody className="font-mono text-ink-2">
            {results.attempts.map((attempt) => (
              <tr key={attempt.id} className="border-t border-line">
                <td className="py-1.5 pr-4">{attempt.enrollmentId}</td>
                <td className="py-1.5 pr-4" data-numeric>
                  #{attempt.number}
                </td>
                <td className="py-1.5 pr-4" data-numeric>
                  v{attempt.version}
                </td>
                <td className={`py-1.5 pr-4 ${attempt.passed ? "text-ok" : "text-warn"}`} data-numeric>
                  {attempt.score} / {attempt.maxScore}
                </td>
                <td className="py-1.5 pr-4" data-numeric>
                  {Math.round(attempt.durationMs / 1000)} s
                </td>
                <td className="py-1.5">
                  {attempt.submittedAt
                    ? new Date(attempt.submittedAt).toLocaleString("fr-FR")
                    : "—"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-surface-1 px-3 py-2.5">
      <dt className="font-mono text-2xs tracking-wide text-ink-3 uppercase">{label}</dt>
      <dd className="mt-0.5 text-base font-semibold tracking-[-0.02em]" data-numeric>
        {value}
      </dd>
    </div>
  );
}
