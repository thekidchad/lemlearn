import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/**
 * Réservation d'un dépôt de logo.
 *
 * Le fichier ne passe jamais par ici : l'API signe une URL de dépôt direct
 * vers le compartiment public, valable cinq minutes, et le navigateur y écrit
 * lui-même.
 */
export async function POST(request: Request) {
  try {
    return NextResponse.json(
      await apiFetch<unknown>("/v1/marque/logo", {
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
