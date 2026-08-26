import Link from "next/link";
import { redirect } from "next/navigation";
import { Logo } from "@/components/brand/logo";
import { SignOutButton } from "@/components/app/sign-out";
import { apiFetch, ApiError, type Me } from "@/lib/api";

/** Ce que voit l'équipe lemlearn, et elle seule. */
const team = [
  { href: "/admin", label: "Organisations", hint: undefined },
  { href: "/admin/emails", label: "Journal des envois", hint: undefined },
  { href: "/admin/gabarits", label: "Gabarits de courriels", hint: undefined },
  { href: "/admin/apprenants", label: "Retrouver un apprenant", hint: undefined },
];

/** Marque la coupure entre les deux jeux d'écrans. */
const separator = { href: "", label: "Mon organisation", hint: undefined };

/** Ce que voit l'équipe de l'organisme. */
const nav = [
  { href: "/pipeline", label: "Pipeline", hint: "G puis P" },
  { href: "/contacts", label: "Contacts", hint: "G puis C" },
  { href: "/catalogue", label: "Catalogue" },
  { href: "/sessions", label: "Sessions" },
  { href: "/questionnaires", label: "Questionnaires" },
  { href: "/qualiopi", label: "Conformité" },
  { href: "/apprenant", label: "Espace apprenant" },
  { href: "/abonnement", label: "Abonnement" },
];

/**
 * Coque de l'organisme.
 *
 * La session est vérifiée ici, une fois, plutôt que dans chaque page : une
 * page qui oublierait de le faire afficherait un écran vide au lieu de
 * rediriger, et le bug serait invisible en développement.
 *
 * Le rôle décide dès cette porte. Un apprenant n'entre pas : ses écrans sont
 * dans une coque à part, et le laisser entrer ici ne produirait qu'un 403 sur
 * la première requête — écran de plantage compris.
 */
export default async function AppLayout({ children }: LayoutProps<"/">) {
  let me: Me;
  try {
    me = await apiFetch<Me>("/v1/me");
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) {
      redirect("/connexion");
    }
    throw error;
  }

  if (me.user.role === "learner") {
    redirect("/apprenant");
  }

  return (
    <div className="flex min-h-full">
      <aside className="hidden w-56 shrink-0 flex-col border-r border-line bg-surface-1/40 lg:flex">
        <div className="flex h-14 items-center px-4">
          <Link href="/pipeline" aria-label="lemlearn, accueil">
            <Logo />
          </Link>
        </div>

        <nav className="flex flex-col gap-0.5 px-2">
          {/* Le compte d'équipe voit ses écrans en premier : il possède bien
              une organisation — vide — mais ce n'est pas ce qu'il vient
              faire. Un client, lui, n'a aucune raison de savoir que ces
              écrans existent. */}
          {(me.user.role === "superadmin" ? [...team, separator, ...nav] : nav).map((item) =>
            item.href === "" ? (
              <p
                key="separator"
                className="mt-3 mb-1 px-2.5 font-mono text-2xs tracking-wide text-ink-3 uppercase"
              >
                {item.label}
              </p>
            ) : (
              <Link
                key={item.href}
                href={item.href}
                className="group flex items-center justify-between rounded-md px-2.5 py-1.5 text-sm text-ink-2 transition-colors duration-[120ms] hover:bg-surface-2 hover:text-ink"
              >
                {item.label}
                {item.hint && (
                  <span className="font-mono text-2xs text-ink-3 opacity-0 transition-opacity group-hover:opacity-100">
                    {item.hint}
                  </span>
                )}
              </Link>
            ),
          )}
        </nav>

        <div className="mt-auto border-t border-line p-3">
          {me.impersonatedBy && (
            // Une impersonation ne peut pas être discrète : elle est visible à
            // l'écran autant qu'au journal.
            <p className="mb-2 rounded-md border border-warn/40 bg-warn/10 px-2 py-1.5 text-2xs text-warn">
              Session ouverte au nom de cet organisme par l&apos;équipe lemlearn.
            </p>
          )}
          <p className="truncate text-xs text-ink">{me.org.name}</p>
          <p className="truncate text-2xs text-ink-3">
            {me.user.firstName} {me.user.lastName} · {me.user.role}
          </p>
          <SignOutButton />
        </div>
      </aside>

      <main className="min-w-0 flex-1">{children}</main>
    </div>
  );
}
