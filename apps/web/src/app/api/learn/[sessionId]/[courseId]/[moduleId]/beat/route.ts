import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/**
 * Relais des signaux du lecteur vers l'API Go.
 *
 * Le navigateur ne peut pas appeler l'API directement : le cookie de session
 * est posé sur notre domaine, pas sur le sien. Ce relais est donc la seule
 * route de l'application appelée depuis le client — et il ne fait que
 * transmettre, sans jamais interpréter ce que le lecteur déclare. C'est le
 * serveur Go qui décide de ce qui compte comme temps suivi.
 */
export async function POST(
  request: Request,
  { params }: { params: Promise<{ sessionId: string; courseId: string; moduleId: string }> },
) {
  const { sessionId, courseId, moduleId } = await params;
  const url = new URL(request.url);
  const contactId = url.searchParams.get("contactId");

  const body = await request.text();
  const path =
    `/v1/learn/${sessionId}/courses/${courseId}/modules/${moduleId}/beat` +
    (contactId ? `?contactId=${encodeURIComponent(contactId)}` : "");

  try {
    const result = await apiFetch<unknown>(path, { method: "POST", body });
    return NextResponse.json(result);
  } catch (error) {
    if (error instanceof ApiError) {
      return NextResponse.json({ accepted: false, reason: error.message }, { status: error.status });
    }
    return NextResponse.json({ accepted: false, reason: "erreur interne" }, { status: 500 });
  }
}
