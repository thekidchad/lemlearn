import Link from "next/link";
import { Logo } from "@/components/brand/logo";
import { ButtonLink } from "@/components/ui/button";

const nav = [
  { href: "#crm", label: "CRM" },
  { href: "#formations", label: "Formations" },
  { href: "#preuve", label: "Preuve" },
  { href: "#conformite", label: "Conformité" },
  { href: "#tarifs", label: "Tarifs" },
];

export function SiteHeader() {
  return (
    <header className="sticky top-0 z-50 border-b border-line/80 bg-surface-0/80 backdrop-blur-xl">
      <div className="mx-auto flex h-14 max-w-6xl items-center gap-8 px-6">
        <Link href="/" aria-label="lemlearn, accueil">
          <Logo />
        </Link>

        <nav className="hidden items-center gap-1 md:flex">
          {nav.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className="rounded-md px-3 py-1.5 text-sm text-ink-2 transition-colors duration-[120ms] hover:bg-surface-2 hover:text-ink"
            >
              {item.label}
            </Link>
          ))}
        </nav>

        <div className="ml-auto flex items-center gap-2">
          <ButtonLink href="/connexion" variant="ghost" size="sm">
            Se connecter
          </ButtonLink>
          <ButtonLink href="/demo" size="sm">
            Demander une démo
          </ButtonLink>
        </div>
      </div>
    </header>
  );
}
