import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/**
 * Ouverture d'une séance de lecture.
 *
 * L'autorisation est demandée au moment où l'apprenant lance la vidéo, jamais
 * au rendu de la page : une autorisation posée d'avance expirerait sur un
 * onglet resté ouvert, et se retrouverait dans le HTML de pages jamais lues.
 */
export async function POST(
  request: Request,
  { params }: { params: Promise<{ sessionId: string; courseId: string; moduleId: string }> },
) {
  const { sessionId, courseId, moduleId } = await params;
  const contactId = new URL(request.url).searchParams.get("contactId");

  const path =
    `/v1/learn/${sessionId}/courses/${courseId}/modules/${moduleId}/playback` +
    (contactId ? `?contactId=${encodeURIComponent(contactId)}` : "");

  try {
    return NextResponse.json(await apiFetch<unknown>(path, { method: "POST" }));
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}
