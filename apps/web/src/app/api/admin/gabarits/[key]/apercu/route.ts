import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/**
 * Aperçu d'un gabarit.
 *
 * Le rendu passe par l'API et non par le navigateur : c'est le même moteur qui
 * validera l'enregistrement, donc ce qu'on voit est exactement ce qui partira.
 */
export async function POST(request: Request, { params }: { params: Promise<{ key: string }> }) {
  const { key } = await params;
  try {
    return NextResponse.json(
      await apiFetch<unknown>(`/v1/admin/gabarits/${encodeURIComponent(key)}/apercu`, {
        method: "POST",
        body: await request.text(),
      }),
    );
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}
