import type { Metadata } from "next";
import { Repertoire } from "@/components/app/repertoire";

export const metadata: Metadata = { title: "Financeurs" };

/** Le répertoire, filtré sur une nature. L'écran porte le nom de ce qu'il contient. */
export default async function Page() {
  return <Repertoire nature="funder" />;
}
