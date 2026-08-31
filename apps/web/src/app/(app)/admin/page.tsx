import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { Bars, Columns, TrendArea } from "@/components/app/charts";
import { apiFetch, ApiError } from "@/lib/api";

export const metadata: Metadata = { title: "Tableau de bord" };

export interface Plan {
  code: string;
  label: string;
  priceCents: number;
  maxLearners: number;
  maxStorageGb: number;
  maxSignatures: number;
  maxVideoHours: number;
  description: string;
}

export interface Usage {
  learners: number;
  files: number;
  sessions: number;
  signatures: number;
  videoMs: number;
  storageMb: number;
}

export interface Dashboard {
  clients: number;
  mrrCents: number;
  learners: number;
  files: number;
  sessions: number;
  signatures: number;
  videoHours: number;
  storageGb: number;
  plans: { code: string; label: string; orgs: number; mrrCents: number }[];
  emailsPerDay: { day: string; sent: number; failed: number }[];
  signaturesPerMonth: { month: string; signatures: number }[];
  overages: { orgId: string; name: string; reasons: string[] }[];
}

export interface Org {
  orgId: string;
  name: string;
  plan: string;
  owner?: string;
  priceCents: number;
  usage?: Usage;
  overage?: string[];
  createdAt: string;
}

export default async function AdminPage() {
  let data: { orgs: Org[]; mrrCents: number; plans: Plan[] };
  try {
    data = await apiFetch("/v1/admin/orgs");
  } catch (error) {
    // Un compte qui n'est pas de l'équipe ne doit pas apprendre que cet écran
    // existe : 403 se rend en 404.
    if (error instanceof ApiError && (error.status === 403 || error.status === 404)) notFound();
    throw error;
  }

  const board = await apiFetch<Dashboard>("/v1/admin/tableau").catch(() => null);
  const overdue = data.orgs.filter((org) => (org.overage?.length ?? 0) > 0);

  return (
    <>
      <header className="flex h-14 items-center gap-4 border-b border-line px-6">
        <h1 className="text-sm font-medium">Tableau de bord</h1>
        <nav className="flex items-center gap-3 text-2xs text-ink-3">
          <Link href="/admin/journal/courriels" className="hover:text-ink">
            Journal
          </Link>
          <Link href="/admin/gabarits" className="hover:text-ink">
            Gabarits
          </Link>
          <Link href="/admin/bibliotheque" className="hover:text-ink">
            Bibliothèque
          </Link>
          {/* Retrouver un apprenant passe désormais par la palette (⌘K) :
              on le cherche en étant déjà occupé à autre chose, pas depuis un
              écran dédié où il faut d'abord se rendre. */}
          <span className="text-ink-3">
            Rechercher <kbd className="font-mono text-[0.625rem]">⌘K</kbd>
          </span>
        </nav>
        <p className="ml-auto font-mono text-2xs text-ink-3" data-numeric>
          {data.orgs.length} client{data.orgs.length > 1 ? "s" : ""} ·{" "}
          {euros(data.mrrCents)} / mois
        </p>
      </header>

      <div className="px-6 py-6">
        {/* Une rangée de tuiles, pas un graphique : cinq nombres du jour se
            lisent mieux posés côte à côte. */}
        <div className="grid grid-cols-2 gap-px overflow-hidden rounded-xl border border-line bg-line lg:grid-cols-5">
          <Stat label="revenu mensuel" value={euros(data.mrrCents)} hero />
          <Stat label="clients" value={String(board?.clients ?? data.orgs.length)} />
          <Stat label="apprenants suivis" value={String(board?.learners ?? 0)} />
          <Stat label="signatures ce mois" value={String(board?.signatures ?? 0)} />
          <Stat
            label="en dépassement"
            value={String(overdue.length)}
            tone={overdue.length > 0}
          />
        </div>

        {board && (
          <div className="mt-6 grid gap-4 lg:grid-cols-2">
            <section className="surface-card p-5">
              <h2 className="text-sm font-medium">Courriels partis</h2>
              <p className="mt-1 text-2xs text-ink-3">
                Trente derniers jours. Les jours creux restent dans la courbe :
                les sauter mentirait sur le rythme.
              </p>
              <div className="mt-4">
                <TrendArea
                  label="Courriels partis par jour"
                  points={board.emailsPerDay.map((day) => ({
                    x: day.day.slice(8) + "/" + day.day.slice(5, 7),
                    y: day.sent,
                  }))}
                />
              </div>
              {board.emailsPerDay.some((day) => day.failed > 0) && (
                <p className="mt-2 flex items-center gap-1.5 text-2xs text-danger">
                  <span aria-hidden>▲</span>
                  {board.emailsPerDay.reduce((sum, day) => sum + day.failed, 0)} envoi(s) en
                  échec —{" "}
                  <Link href="/admin/journal/courriels" className="underline">
                    voir le journal
                  </Link>
                </p>
              )}
            </section>

            <section className="surface-card p-5">
              <h2 className="text-sm font-medium">Signatures par mois</h2>
              <p className="mt-1 text-2xs text-ink-3">
                Tous clients confondus. C&apos;est l&apos;usage qui suit le mieux
                l&apos;activité réelle des organismes.
              </p>
              <div className="mt-4">
                <Columns
                  label="Signatures par mois"
                  bars={board.signaturesPerMonth.map((month) => ({
                    x: month.month.slice(5) + "/" + month.month.slice(2, 4),
                    y: month.signatures,
                  }))}
                />
              </div>
            </section>

            <section className="surface-card p-5">
              <h2 className="text-sm font-medium">Répartition par formule</h2>
              <div className="mt-4">
                <Bars
                  rows={board.plans.map((plan) => ({
                    label: plan.label,
                    value: plan.orgs,
                    note: plan.mrrCents > 0 ? euros(plan.mrrCents) : undefined,
                  }))}
                  suffix=" client(s)"
                />
              </div>
            </section>

            <section className="surface-card p-5">
              <h2 className="text-sm font-medium">Volumes hébergés</h2>
              <div className="mt-4">
                <Bars
                  rows={[
                    { label: "Dossiers", value: board.files },
                    { label: "Sessions", value: board.sessions },
                    { label: "Vidéo (h)", value: board.videoHours },
                    { label: "Stockage (Go)", value: board.storageGb },
                  ]}
                />
              </div>
            </section>
          </div>
        )}

        {overdue.length > 0 && (
          <p className="mt-6 rounded-lg border border-warn/40 bg-warn/10 px-3 py-2 text-2xs text-warn">
            {overdue.length === 1
              ? "Une organisation dépasse"
              : `${overdue.length} organisations dépassent`}{" "}
            leur formule. Un dépassement ne coupe rien : il se règle en changeant
            de palier, pas en bloquant une session de formation.
          </p>
        )}

        {/* La liste des organismes est en tête de console, pas au bas d'une
            page de graphiques : c'est l'écran qu'on ouvre, pas celui qu'on
            découvre en défilant. */}
        <p className="mt-8 text-xs text-ink-3">
          Les organismes clients, leur formule et l&apos;accès à leur espace sont
          dans{" "}
          <Link href="/admin/organismes" className="underline hover:text-ink">
            Organismes
          </Link>
          .
        </p>
      </div>
    </>
  );
}

function Stat({
  label,
  value,
  tone,
  hero,
}: {
  label: string;
  value: string;
  tone?: boolean;
  hero?: boolean;
}) {
  return (
    <div className="bg-surface-1 px-4 py-3.5">
      <p className="font-mono text-2xs tracking-wide text-ink-3 uppercase">{label}</p>
      <p
        className={`mt-1 font-semibold tracking-[-0.02em] ${hero ? "text-3xl" : "text-lg"} ${
          tone ? "text-warn" : ""
        }`}
        data-numeric
      >
        {value}
      </p>
    </div>
  );
}

function euros(cents: number): string {
  return new Intl.NumberFormat("fr-FR", {
    style: "currency",
    currency: "EUR",
    maximumFractionDigits: 0,
  }).format(cents / 100);
}
