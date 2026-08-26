import type { Metadata } from "next";
import Link from "next/link";
import { LibraryEditor } from "@/components/app/library-editor";
import { apiFetch } from "@/lib/api";

export const metadata: Metadata = { title: "Bibliothèque de formations" };

export interface LibraryCourse {
  id: string;
  title: string;
  summary?: string;
  goal?: string;
  objectives?: string[];
  prerequisites?: string;
  audience?: string;
  means?: string;
  assessment?: string;
  sanction?: string;
  accessibility?: string;
  durationHours: number;
  tags?: string[];
  published: boolean;
  updatedAt: string;
}

/**
 * Bibliothèque de formations mise à disposition des organismes.
 *
 * Ils n'y prennent pas une référence mais une copie : la convention qu'un
 * organisme signe décrit ce qu'il dispense réellement, et une formation qui
 * changerait sous ses pieds parce que nous l'avons remaniée rendrait ses
 * documents faux rétroactivement.
 */
export default async function LibraryPage() {
  const { courses } = await apiFetch<{ courses: LibraryCourse[] }>("/v1/admin/bibliotheque");
  const published = courses.filter((course) => course.published);

  return (
    <>
      <header className="flex h-14 items-center gap-3 border-b border-line px-6">
        <Link href="/admin" className="text-xs text-ink-3 hover:text-ink">
          Tableau de bord
        </Link>
        <span className="text-ink-3">/</span>
        <h1 className="text-sm font-medium">Bibliothèque de formations</h1>
        <span className="ml-auto font-mono text-2xs text-ink-3" data-numeric>
          {published.length} publiée{published.length > 1 ? "s" : ""} sur {courses.length}
        </span>
      </header>

      <div className="mx-auto max-w-4xl px-6 py-6">
        <p className="text-xs text-ink-2">
          Ces formations apparaissent dans le catalogue des organismes, qui les
          importent d&apos;un clic. L&apos;import est une <strong>copie</strong> :
          elle devient la leur, ils l&apos;adaptent à leurs moyens et à leur
          public, et vos remaniements ultérieurs ne la touchent plus.
        </p>

        <LibraryEditor courses={courses} />
      </div>
    </>
  );
}
