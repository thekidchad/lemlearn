import type { Metadata } from "next";
import Link from "next/link";
import { apiFetch } from "@/lib/api";

export const metadata: Metadata = { title: "Conformité" };

interface Gap {
  reference: string;
  title: string;
  percent: number;
  missing: string[];
}

interface Dashboard {
  dossiers: number;
  complets: number;
  tauxDeCompletude: number;
  piecesManquantes: Record<string, number>;
  dossiersIncomplets: Gap[];
}

/**
 * État de conformité de l'organisation.
 *
 * L'écran répond à la question qu'on se pose la veille d'un audit : combien de
 * dossiers tiennent debout, lesquels non, et ce qui leur manque. Les dossiers
 * les plus incomplets viennent en premier — c'est par eux qu'un contrôle
 * commence.
 */
export default async function QualiopiPage() {
  const data = await apiFetch<Dashboard>("/v1/qualiopi");
  const pieces = Object.entries(data.piecesManquantes ?? {}).sort(([, a], [, b]) => b - a);

  return (
    <>
      <header className="flex h-14 items-center justify-between border-b border-line px-6">
        <h1 className="text-sm font-medium">Conformité</h1>
        <p className="font-mono text-2xs text-ink-3" data-numeric>
          {data.complets} / {data.dossiers} dossiers complets
        </p>
      </header>

      <div className="mx-auto max-w-4xl px-6 py-6">
        <div className="grid grid-cols-3 gap-px overflow-hidden rounded-xl border border-line bg-line">
          <Stat label="dossiers suivis" value={String(data.dossiers)} />
          <Stat label="complets" value={String(data.complets)} />
          <Stat
            label="taux de complétude"
            value={`${data.tauxDeCompletude} %`}
            tone={data.tauxDeCompletude < 80}
          />
        </div>

        {pieces.length > 0 && (
          <section className="mt-6">
            <h2 className="text-xs font-medium">Ce qui manque le plus souvent</h2>
            <p className="mt-1 text-2xs text-ink-3">
              {/* Une pièce absente partout est un défaut de procédure, pas un
                  oubli : c'est la seule lecture qui dit quoi corriger une fois
                  pour toutes. */}
              Une pièce qui manque sur presque tous les dossiers est un défaut de
              procédure, pas un oubli.
            </p>
            <ul className="mt-3 space-y-1.5">
              {pieces.map(([piece, count]) => (
                <li key={piece} className="flex items-center gap-3">
                  <span className="w-56 shrink-0 truncate text-xs text-ink-2">{piece}</span>
                  <span className="h-1 flex-1 overflow-hidden rounded-full bg-surface-3">
                    <span
                      className="block h-full bg-warn"
                      style={{ width: `${Math.min((count / Math.max(data.dossiers, 1)) * 100, 100)}%` }}
                    />
                  </span>
                  <span className="w-16 shrink-0 text-right font-mono text-2xs text-ink-3" data-numeric>
                    {count} dossier{count > 1 ? "s" : ""}
                  </span>
                </li>
              ))}
            </ul>
          </section>
        )}

        <h2 className="mt-8 text-xs font-medium">Dossiers incomplets</h2>
        {data.dossiersIncomplets.length === 0 ? (
          <p className="mt-2 text-xs text-ok">
            Tous les dossiers sont complets. C&apos;est l&apos;état dans lequel un
            audit ne prend pas de temps.
          </p>
        ) : (
          <div className="mt-2 space-y-px overflow-hidden rounded-xl border border-line bg-line">
            {data.dossiersIncomplets.map((gap) => (
              <div key={gap.reference} className="bg-surface-1 px-4 py-3">
                <div className="flex items-center justify-between gap-4">
                  <p className="min-w-0 truncate text-sm">
                    <span className="mr-2 font-mono text-2xs text-ink-3">{gap.reference}</span>
                    {gap.title}
                  </p>
                  <span
                    className={`shrink-0 font-mono text-2xs ${
                      gap.percent >= 60 ? "text-warn" : "text-danger"
                    }`}
                    data-numeric
                  >
                    {gap.percent} %
                  </span>
                </div>
                <p className="mt-1 text-2xs text-ink-3">Manque : {gap.missing.join(", ")}</p>
              </div>
            ))}
          </div>
        )}

        <p className="mt-6 text-2xs text-ink-3">
          Le décompte est déduit du journal d&apos;audit de chaque dossier, jamais
          d&apos;un compteur tenu à la main : un indicateur de conformité qui se
          trompe à la hausse rassure la veille d&apos;un audit sur des pièces qui
          n&apos;existent pas.{" "}
          <Link href="/pipeline" className="underline hover:text-ink">
            Voir le pipeline
          </Link>
        </p>
      </div>
    </>
  );
}

function Stat({ label, value, tone }: { label: string; value: string; tone?: boolean }) {
  return (
    <div className="bg-surface-1 px-4 py-3.5">
      <p className="font-mono text-2xs tracking-wide text-ink-3 uppercase">{label}</p>
      <p
        className={`mt-1 text-lg font-semibold tracking-[-0.02em] ${tone ? "text-warn" : ""}`}
        data-numeric
      >
        {value}
      </p>
    </div>
  );
}
