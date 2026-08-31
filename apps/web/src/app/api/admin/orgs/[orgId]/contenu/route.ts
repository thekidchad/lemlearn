import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/**
 * Relais du contenu d'un organisme, pour l'équipe.
 *
 * Une seule route pour les quatre natures : le composant passe `vue`, et
 * l'adresse en aval s'en déduit. Quatre relais identiques n'auraient rien
 * apporté qu'un endroit de plus où se tromper.
 */
const VUES: Record<string, string> = {
  repertoire: "repertoire",
  sessions: "sessions",
  formations: "formations",
  dossiers: "dossiers",
};

export async function GET(
  request: Request,
  { params }: { params: Promise<{ orgId: string }> },
) {
  const { orgId } = await params;
  const search = new URL(request.url).searchParams;
  const vue = VUES[search.get("vue") ?? "repertoire"];
  if (!vue) return NextResponse.json({ error: "vue inconnue" }, { status: 400 });

  const query = new URLSearchParams();
  for (const clef of ["kind", "etape", "limite", "curseur"]) {
    const valeur = search.get(clef);
    if (valeur) query.set(clef, valeur);
  }

  try {
    return NextResponse.json(
      await apiFetch<unknown>(`/v1/admin/orgs/${orgId}/${vue}?${query}`),
    );
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}
