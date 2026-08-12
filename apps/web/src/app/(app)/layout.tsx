import Link from "next/link";
import { redirect } from "next/navigation";
import { Logo } from "@/components/brand/logo";
import { SignOutButton } from "@/components/app/sign-out";
import { apiFetch, ApiError, type Me } from "@/lib/api";

const nav = [
  { href: "/pipeline", label: "Pipeline", hint: "G puis P" },
  { href: "/contacts", label: "Contacts", hint: "G puis C" },
  { href: "/catalogue", label: "Catalogue" },
  { href: "/sessions", label: "Sessions" },
  { href: "/apprenant", label: "Espace apprenant" },
];

/**
 * Coque de l'application.
 *
 * La session est vérifiée ici, une fois, plutôt que dans chaque page : une
 * page qui oublierait de le faire afficherait un écran vide au lieu de
 * rediriger, et le bug serait invisible en développement.
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

  return (
    <div className="flex min-h-full">
      <aside className="hidden w-56 shrink-0 flex-col border-r border-line bg-surface-1/40 lg:flex">
        <div className="flex h-14 items-center px-4">
          <Link href="/pipeline" aria-label="lemlearn, accueil">
            <Logo />
          </Link>
        </div>

        <nav className="flex flex-col gap-0.5 px-2">
          {nav.map((item) => (
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
          ))}
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
