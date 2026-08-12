import type { Metadata } from "next";
import Link from "next/link";
import { apiFetch } from "@/lib/api";

export const metadata: Metadata = { title: "Sessions" };

interface Session {
  id: string;
  courseId: string;
  title: string;
  mode: "onsite" | "virtual" | "async" | "blended";
  startsAt: string;
  endsAt: string;
  location?: string;
  tags?: string[];
  closed: boolean;
}

const MODES: Record<Session["mode"], string> = {
  onsite: "Présentiel",
  virtual: "Classe virtuelle",
  async: "Asynchrone",
  blended: "Mixte",
};

export default async function SessionsPage() {
  const { sessions } = await apiFetch<{ sessions: Session[] | null }>("/v1/sessions");
  const rows = sessions ?? [];

  return (
    <>
      <header className="flex h-14 items-center gap-2.5 border-b border-line px-6">
        <h1 className="text-sm font-medium">Sessions</h1>
        <span className="rounded-md border border-line bg-surface-2 px-1.5 py-0.5 font-mono text-2xs text-ink-3">
          {rows.length}
        </span>
      </header>

      {rows.length === 0 ? (
        <p className="px-6 py-16 text-center text-xs text-ink-3">
          Aucune session planifiée. Une session est une occurrence datée d&apos;une
          formation : c&apos;est elle qui porte les inscriptions et l&apos;émargement.
        </p>
      ) : (
        <ul className="divide-y divide-line">
          {rows.map((session) => (
            <li key={session.id}>
              <Link
                href={`/sessions/${session.id}`}
                className="flex flex-wrap items-center gap-x-6 gap-y-2 px-6 py-3.5 transition-colors duration-[120ms] hover:bg-surface-1"
              >
                <span className="min-w-40">
                  <span className="block text-sm">{session.title}</span>
                  <span className="block text-2xs text-ink-3">{MODES[session.mode]}</span>
                </span>

                <span className="font-mono text-xs text-ink-2" data-numeric>
                  {formatRange(session.startsAt, session.endsAt)}
                </span>

                {session.location && (
                  <span className="truncate text-xs text-ink-3">{session.location}</span>
                )}

                <span className="ml-auto flex items-center gap-1.5">
                  {session.tags?.map((tag) => (
                    <span
                      key={tag}
                      className="rounded border border-line px-1.5 py-0.5 text-2xs text-ink-3"
                    >
                      {tag}
                    </span>
                  ))}
                  <span
                    className={`size-1.5 rounded-full ${session.closed ? "bg-ink-3" : "bg-ok"}`}
                    title={session.closed ? "Clôturée" : "Ouverte"}
                  />
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </>
  );
}

function formatRange(start: string, end: string): string {
  const format = new Intl.DateTimeFormat("fr-FR", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    timeZone: "Europe/Paris",
  });
  const from = format.format(new Date(start));
  const to = format.format(new Date(end));
  return from === to ? from : `${from} – ${to}`;
}
