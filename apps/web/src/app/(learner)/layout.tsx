import Link from "next/link";
import { redirect } from "next/navigation";
import { SignOutButton } from "@/components/app/sign-out";
import { Logo } from "@/components/brand/logo";
import { apiFetch, ApiError, type Me } from "@/lib/api";

/**
 * Coque de l'espace apprenant.
 *
 * Elle est séparée de celle de l'organisme pour une raison de fond : un
 * apprenant n'a rien à voir des écrans de gestion, et une coque unique
 * l'obligeait à croiser des liens qui lui répondaient 403. Ici, il n'y a rien
 * à cacher — il n'y a que son parcours.
 *
 * L'équipe de l'organisme y passe aussi, pour consulter le parcours d'un
 * apprenant : elle garde alors un retour vers ses propres écrans.
 */
export default async function LearnerLayout({ children }: LayoutProps<"/">) {
  let me: Me;
  try {
    me = await apiFetch<Me>("/v1/me");
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) {
      redirect("/connexion");
    }
    throw error;
  }

  const staff = me.user.role !== "learner";

  return (
    <div className="flex min-h-full">
      <aside className="hidden w-56 shrink-0 flex-col border-r border-line bg-surface-1/40 lg:flex">
        <div className="flex h-14 items-center px-4">
          <Link href="/apprenant" aria-label="lemlearn, accueil">
            <Logo />
          </Link>
        </div>

        <nav className="flex flex-col gap-0.5 px-2">
          <Link
            href="/apprenant"
            className="rounded-md px-2.5 py-1.5 text-sm text-ink-2 transition-colors duration-[120ms] hover:bg-surface-2 hover:text-ink"
          >
            Mon parcours
          </Link>
          {staff && (
            <Link
              href="/pipeline"
              className="rounded-md px-2.5 py-1.5 text-sm text-ink-3 transition-colors duration-[120ms] hover:bg-surface-2 hover:text-ink"
            >
              ← Retour à l&apos;organisme
            </Link>
          )}
        </nav>

        <div className="mt-auto border-t border-line p-3">
          {me.impersonatedBy && (
            <p className="mb-2 rounded-md border border-warn/40 bg-warn/10 px-2 py-1.5 text-2xs text-warn">
              Session ouverte au nom de cet organisme par l&apos;équipe lemlearn.
            </p>
          )}
          <p className="truncate text-xs text-ink">{me.org.name}</p>
          <p className="truncate text-2xs text-ink-3">
            {me.user.firstName} {me.user.lastName}
          </p>
          <SignOutButton />
        </div>
      </aside>

      <main className="min-w-0 flex-1">{children}</main>
    </div>
  );
}
