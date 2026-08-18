import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/**
 * Relais des réponses au questionnaire de satisfaction à froid.
 *
 * La route de l'API est publique — la légitimité vient du jeton du lien, pas
 * d'une session — mais elle n'accepte que l'origine de l'application : passer
 * par le serveur évite d'avoir à ouvrir le CORS pour une seule route.
 */
export async function POST(
  request: Request,
  { params }: { params: Promise<{ token: string }> },
) {
  const { token } = await params;
  const body = await request.text();

  try {
    const result = await apiFetch<unknown>(
      `/v1/satisfaction/${encodeURIComponent(token)}`,
      { method: "POST", body },
    );
    return NextResponse.json(result);
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}
