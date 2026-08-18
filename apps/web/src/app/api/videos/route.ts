import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/**
 * Relais du dépôt vidéo.
 *
 * Sans paramètre, réserve un emplacement et renvoie l'URL de dépôt direct sur
 * S3 ; avec `id`, signale que le fichier est arrivé et déclenche le
 * transcodage. Le fichier lui-même ne passe jamais par ici : une heure de
 * vidéo ne traverse pas une fonction serverless.
 */
export async function POST(request: Request) {
  const id = new URL(request.url).searchParams.get("id");
  const body = await request.text();

  const path = id ? `/v1/videos/${encodeURIComponent(id)}/uploaded` : "/v1/videos";
  try {
    return NextResponse.json(await apiFetch<unknown>(path, { method: "POST", body }));
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}
