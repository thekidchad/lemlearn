import Link from "next/link";
import { Logo } from "@/components/brand/logo";
import { ButtonLink } from "@/components/ui/button";

const columns = [
  {
    title: "Produit",
    links: [
      { href: "#produit", label: "Fonctionnalités" },
      { href: "#preuve", label: "Chaîne de preuve" },
      { href: "#tarifs", label: "Tarifs" },
      { href: "/demo", label: "Démo" },
    ],
  },
  {
    title: "Conformité",
    links: [
      { href: "#conformite", label: "Qualiopi" },
      { href: "/rgpd", label: "RGPD" },
      { href: "/securite", label: "Sécurité" },
      { href: "/sous-traitants", label: "Sous-traitants" },
    ],
  },
  {
    title: "Légal",
    links: [
      { href: "/mentions-legales", label: "Mentions légales" },
      { href: "/cgu", label: "CGU" },
      { href: "/cgv", label: "CGV" },
      { href: "/cookies", label: "Cookies" },
    ],
  },
];

export function SiteFooter() {
  return (
    <footer className="border-t border-line bg-surface-1/40">
      <div className="mx-auto max-w-6xl px-6 py-16">
        <div className="surface-card flex flex-col items-start justify-between gap-6 p-6 sm:flex-row sm:items-center">
          <div>
            <h2 className="text-lg font-semibold tracking-[-0.03em]">
              Voir la chaîne de preuve sur vos propres dossiers.
            </h2>
            <p className="mt-1.5 text-xs text-ink-2">
              30 minutes, en visio, avec une session réelle de votre catalogue.
            </p>
          </div>
          <ButtonLink href="/demo" size="lg" className="shrink-0">
            Demander une démo
          </ButtonLink>
        </div>

        <div className="mt-14 grid gap-10 sm:grid-cols-2 lg:grid-cols-4">
          <div>
            <Logo />
            <p className="mt-3 max-w-56 text-xs leading-relaxed text-ink-3">
              Le CRM et le LMS des organismes de formation qui doivent produire des
              preuves.
            </p>
          </div>

          {columns.map((column) => (
            <div key={column.title}>
              <p className="text-2xs font-medium tracking-wide text-ink-3 uppercase">
                {column.title}
              </p>
              <ul className="mt-3 space-y-2">
                {column.links.map((link) => (
                  <li key={link.href}>
                    <Link
                      href={link.href}
                      className="text-xs text-ink-2 transition-colors duration-[120ms] hover:text-ink"
                    >
                      {link.label}
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>

        <div className="mt-14 flex flex-col justify-between gap-3 border-t border-line pt-6 sm:flex-row">
          <p className="text-2xs text-ink-3">
            © {new Date().getFullYear()} lemlearn — données hébergées en France.
          </p>
          <p className="font-mono text-2xs text-ink-3">
            Horodatage RFC 3161 · scellement PAdES · journal d&apos;audit chaîné
          </p>
        </div>
      </div>
    </footer>
  );
}
