import type { Metadata } from "next";
import Link from "next/link";
import { redirect } from "next/navigation";
import { Logo } from "@/components/brand/logo";
import { CommandPalette } from "@/components/app/command-palette";
import { LeaveImpersonation } from "@/components/app/leave-impersonation";
import { OrgBrand, brandStyle } from "@/components/brand/org-brand";
import { SignOutButton } from "@/components/app/sign-out";
import { cookies } from "next/headers";
import { ThemeSwitch } from "@/components/app/theme-switch";
import { apiFetch, ApiError, type Me } from "@/lib/api";
import { THEME_COOKIE, type Theme } from "@/lib/theme";

/**
 * La navigation, groupée par moment de travail plutôt que par entité.
 *
 * On ne se demande pas « où sont les contacts » mais « où j'en suis avec ce
 * client » : vendre, former, prouver. La conformité ferme la marche parce que
 * c'est là qu'on va la veille d'un audit.
 */
const groups: { title?: string; items: { href: string; label: string; glyph: string }[] }[] = [
  {
    items: [
      { href: "/pipeline", label: "Pipeline", glyph: "◧" },
      { href: "/stagiaires", label: "Stagiaires", glyph: "◍" },
      { href: "/entreprises", label: "Entreprises", glyph: "▢" },
      { href: "/financeurs", label: "Financeurs", glyph: "◇" },
    ],
  },
  {
    title: "Former",
    items: [
      { href: "/catalogue", label: "Catalogue", glyph: "▤" },
      { href: "/sessions", label: "Sessions", glyph: "◷" },
      { href: "/questionnaires", label: "Questionnaires", glyph: "◇" },
      { href: "/apprenant", label: "Espace apprenant", glyph: "▷" },
    ],
  },
  {
    title: "Prouver",
    items: [{ href: "/qualiopi", label: "Conformité", glyph: "✓" }],
  },
  {
    title: "Compte",
    items: [
      { href: "/organisme", label: "Votre organisme", glyph: "◉" },
      { href: "/abonnement", label: "Abonnement", glyph: "◈" },
    ],
  },
];

/** Ce que voit l'équipe lemlearn, et elle seule. */
const teamGroup = {
  title: "Équipe lemlearn",
  items: [
    { href: "/admin", label: "Tableau de bord", glyph: "◰" },
    { href: "/admin/donnees", label: "Toutes les données", glyph: "▤" },
    { href: "/admin/emails", label: "Journal des envois", glyph: "✉" },
    { href: "/admin/gabarits", label: "Gabarits", glyph: "❏" },
    { href: "/admin/bibliotheque", label: "Bibliothèque", glyph: "▥" },
  ],
};

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
/**
 * Le titre et l'icône de l'onglet suivent l'organisme.
 *
 * C'est le dernier endroit où notre nom transparaissait : un apprenant qui
 * garde son onglet ouvert toute la journée y lirait « lemlearn » sans jamais
 * comprendre pourquoi. L'équipe, elle, reste chez elle.
 */
export async function generateMetadata(): Promise<Metadata> {
  try {
    const me = await apiFetch<Me>("/v1/me");
    const team = me.user.role === "superadmin" && !me.impersonatedBy;
    if (team) return { title: { default: "lemlearn", template: "%s · lemlearn" } };
    return {
      title: { default: me.brand.name, template: `%s · ${me.brand.name}` },
      ...(me.brand.logoUrl ? { icons: { icon: me.brand.logoUrl } } : {}),
    };
  } catch {
    // Sans session, la coque redirige de toute façon vers la connexion.
    return {};
  }
}

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

  const theme = ((await cookies()).get(THEME_COOKIE)?.value ?? "system") as Theme;

  // L'équipe lemlearn travaille sous sa propre enseigne : c'est elle qui gère
  // les organismes, et lui afficher la marque d'un client la tromperait sur
  // l'endroit où elle se trouve. Tous les autres voient la leur.
  const team = me.user.role === "superadmin" && !me.impersonatedBy;
  const sections = me.user.role === "superadmin" ? [teamGroup, ...groups] : groups;

  return (
    <div className="flex min-h-dvh" style={team ? undefined : brandStyle(me.brand)}>
      {/* La colonne reste en place pendant que le contenu défile : c'est ce
          qui fait qu'on ne se perd pas dans un écran long. */}
      <aside className="sticky top-0 hidden h-dvh w-60 shrink-0 flex-col border-r border-line bg-surface-1 lg:flex">
        <div className="flex h-12 items-center px-3">
          <Link
            href="/pipeline"
            aria-label={`${team ? "lemlearn" : me.brand.name}, accueil`}
            className="min-w-0 px-1"
          >
            {team ? <Logo /> : <OrgBrand brand={me.brand} />}
          </Link>
        </div>

        <nav className="flex flex-1 flex-col gap-0.5 overflow-y-auto px-2 pb-3">
          {team && (
            // La recherche se pose par-dessus l'écran plutôt que de le
            // remplacer : on cherche un organisme en étant déjà occupé à
            // autre chose.
            <div className="mb-2">
              <CommandPalette />
            </div>
          )}
          {sections.map((section, index) => (
            <div key={section.title ?? index} className={index > 0 ? "mt-4" : ""}>
              {section.title && <p className="eyebrow mb-1 px-2">{section.title}</p>}
              {section.items.map((item) => (
                <Link key={item.href} href={item.href} className="nav-item">
                  <span aria-hidden className="w-4 text-center text-ink-3">
                    {item.glyph}
                  </span>
                  {item.label}
                </Link>
              ))}
            </div>
          ))}
        </nav>

        <div className="border-t border-line p-3">
          {me.impersonatedBy && (
            // Une impersonation ne peut pas être discrète : elle est visible à
            // l'écran autant qu'au journal.
            <p className="mb-2 rounded-md border border-warn/40 bg-warn/10 px-2 py-1.5 text-2xs text-warn">
              Session ouverte au nom de cet organisme par l&apos;équipe lemlearn.
              <LeaveImpersonation />
            </p>
          )}

          <div className="flex items-center gap-2">
            <span className="flex size-7 shrink-0 items-center justify-center rounded-md bg-accent-dim text-2xs font-medium text-accent-ink">
              {initials(me.user.firstName, me.user.lastName)}
            </span>
            <div className="min-w-0 flex-1">
              <p className="truncate text-xs font-medium">{me.org.name}</p>
              <p className="truncate text-2xs text-ink-3">
                {me.user.firstName} {me.user.lastName}
              </p>
            </div>
          </div>

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

/** initials compose la pastille du compte : deux lettres, pas d'avatar. */
function initials(firstName: string, lastName: string): string {
  return `${firstName.charAt(0)}${lastName.charAt(0)}`.toUpperCase() || "—";
}
