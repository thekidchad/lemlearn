import type { Metadata } from "next";
import Link from "next/link";
import { PlatformData } from "@/components/app/platform-data";

export const metadata: Metadata = { title: "Toutes les données" };

/**
 * Toute la plateforme, tous organismes confondus.
 *
 * C'est la vue que cherche l'équipe quand elle veut savoir ce que contient le
 * produit, et non ce que contient un client donné. Chaque ligne dit à quel
 * organisme elle appartient — sans quoi la liste serait un tas.
 */
export default async function DonneesPage() {
  return (
    <>
      <header className="flex h-14 items-center gap-3 border-b border-line px-6">
        <Link href="/admin" className="text-xs text-ink-3 hover:text-ink">
          Organisations
        </Link>
        <span className="text-ink-3">/</span>
        <h1 className="text-sm font-medium">Toutes les données</h1>
      </header>

      <div className="px-6 py-6">
        <p className="mb-5 max-w-2xl text-xs text-ink-2">
          Tout ce que contient la plateforme, chez tous les organismes. Chaque
          consultation est journalisée chez le client concerné, et modifier passe
          par l&apos;ouverture d&apos;une session sur sa fiche.
        </p>
        <PlatformData />
      </div>
    </>
  );
}
