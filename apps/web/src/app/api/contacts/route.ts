import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/** Relais paginé des contacts : le composant client suit le curseur. */
export async function GET(request: Request) {
  const params = new URL(request.url).searchParams;
  const query = new URLSearchParams();
  for (const clef of ["kind", "limite", "curseur"]) {
    const valeur = params.get(clef);
    if (valeur) query.set(clef, valeur);
  }
  try {
    return NextResponse.json(await apiFetch<unknown>(`/v1/contacts?${query}`));
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}
