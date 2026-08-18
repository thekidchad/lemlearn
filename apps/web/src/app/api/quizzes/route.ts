import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/**
 * Relais de l'éditeur de questionnaires.
 *
 * Sans paramètre, la requête enregistre une version ; avec `id` et `version`,
 * elle la publie — deux routes de l'API, un seul point d'entrée côté client.
 */
export async function POST(request: Request) {
  const query = new URL(request.url).searchParams;
  const id = query.get("id");
  const version = query.get("version");

  const path =
    id && version
      ? `/v1/quizzes/${encodeURIComponent(id)}/versions/${encodeURIComponent(version)}/publish`
      : "/v1/quizzes";

  try {
    const body = id && version ? undefined : await request.text();
    return NextResponse.json(await apiFetch<unknown>(path, { method: "POST", body }));
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}
