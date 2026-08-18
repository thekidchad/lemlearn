import { NextResponse } from "next/server";
import { apiText, ApiError } from "@/lib/api";

/**
 * Relais du manifeste HLS.
 *
 * Le manifeste seul passe par ici — quelques kilo-octets de texte ; les
 * segments vidéo partent directement au CDN, avec les URL signées que l'API a
 * écrites dedans. Le segment de chemin porte l'extension `.m3u8` parce que
 * certains lecteurs déduisent le format de l'URL avant de lire l'en-tête, et
 * parce que les sous-manifestes s'y résolvent en relatif.
 */
export async function GET(
  request: Request,
  { params }: { params: Promise<{ sessionId: string; courseId: string; moduleId: string }> },
) {
  const { sessionId, courseId, moduleId } = await params;
  const asked = new URL(request.url).searchParams;

  const query = new URLSearchParams();
  for (const key of ["rendu", "contactId"]) {
    const value = asked.get(key);
    if (value) query.set(key, value);
  }

  const path =
    `/v1/learn/${sessionId}/courses/${courseId}/modules/${moduleId}/manifest.m3u8` +
    (query.size > 0 ? `?${query}` : "");

  try {
    const { body, contentType } = await apiText(path);
    return new NextResponse(body, {
      headers: { "Content-Type": contentType, "Cache-Control": "no-store" },
    });
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}
