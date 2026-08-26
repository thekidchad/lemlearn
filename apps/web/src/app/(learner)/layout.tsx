import Link from "next/link";
import { redirect } from "next/navigation";
import { SignOutButton } from "@/components/app/sign-out";
import { Logo } from "@/components/brand/logo";
import { cookies } from "next/headers";
import { ThemeSwitch } from "@/components/app/theme-switch";
import { apiFetch, ApiError, type Me } from "@/lib/api";
import { THEME_COOKIE, type Theme } from "@/lib/theme";

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

  const theme = ((await cookies()).get(THEME_COOKIE)?.value ?? "system") as Theme;

  return (
    <div className="flex min-h-dvh">
      {/* La colonne reste en place pendant que le contenu défile : c'est ce
          qui fait qu'on ne se perd pas dans un écran long. */}
      <aside className="sticky top-0 hidden h-dvh w-60 shrink-0 flex-col border-r border-line bg-surface-1 lg:flex">
        <div className="flex h-12 items-center px-3">
          <Link href="/apprenant" aria-label="lemlearn, accueil" className="px-1">
            <Logo />
          </Link>
        </div>

        <nav className="flex flex-1 flex-col gap-0.5 px-2">
          <Link href="/apprenant" className="nav-item">
            <span aria-hidden className="w-4 text-center text-ink-3">
              ▷
            </span>
            Mon parcours
          </Link>
          {staff && (
            <Link href="/pipeline" className="nav-item">
              <span aria-hidden className="w-4 text-center text-ink-3">
                ←
              </span>
              Retour à l&apos;organisme
            </Link>
          )}
        </nav>

        <div className="border-t border-line p-3">
          {me.impersonatedBy && (
            <p className="mb-2 rounded-md border border-warn/40 bg-warn/10 px-2 py-1.5 text-2xs text-warn">
              Session ouverte au nom de cet organisme par l&apos;équipe lemlearn.
            </p>
          )}
          <p className="truncate text-xs font-medium">{me.org.name}</p>
          <p className="truncate text-2xs text-ink-3">
            {me.user.firstName} {me.user.lastName}
          </p>
          <div className="mt-2.5 flex items-center gap-2">
            <ThemeSwitch current={theme} />
            <div className="flex-1">
              <SignOutButton />
            </div>
          </div>
        </div>
      </aside>

      <main className="min-w-0 flex-1">{children}</main>
    </div>
  );
}
