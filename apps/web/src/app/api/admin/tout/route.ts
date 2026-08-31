import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/** Relais de la vue plateforme, tous organismes confondus. */
export async function GET(request: Request) {
  const search = new URL(request.url).searchParams;
  const query = new URLSearchParams();
  for (const clef of ["vue", "etape", "limite", "curseur"]) {
    const valeur = search.get(clef);
    if (valeur) query.set(clef, valeur);
  }
  try {
    return NextResponse.json(await apiFetch<unknown>(`/v1/admin/tout?${query}`));
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}
