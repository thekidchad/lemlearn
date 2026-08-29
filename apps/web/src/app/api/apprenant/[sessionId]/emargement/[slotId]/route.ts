import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/**
 * Relais de l'émargement d'un apprenant.
 *
 * Le bouton est un composant client — il doit répondre sans recharger la page —
 * et ne peut donc pas porter le cookie de session vers l'API. Ce relais le fait
 * pour lui. La fiche émargée, elle, n'est jamais transmise : l'API la déduit du
 * compte connecté, sans quoi un apprenant pourrait émarger pour un camarade.
 */
export async function POST(
  request: Request,
  { params }: { params: Promise<{ sessionId: string; slotId: string }> },
) {
  const { sessionId, slotId } = await params;
  // Le paramètre `contactId` n'est honoré par l'API que pour un formateur : il
  // sert à émarger depuis l'espace d'un apprenant qu'on accompagne.
  const contactId = new URL(request.url).searchParams.get("contactId");
  const query = contactId ? `?contactId=${encodeURIComponent(contactId)}` : "";

  try {
    return NextResponse.json(
      await apiFetch<unknown>(
        `/v1/learn/${encodeURIComponent(sessionId)}/emargement/${encodeURIComponent(slotId)}${query}`,
        { method: "POST" },
      ),
    );
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}
