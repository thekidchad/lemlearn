import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

/**
 * Relais d'une pièce signée vers son signataire.
 *
 * Le PDF est retransmis tel quel : l'empreinte affichée à côté du bouton est
 * celle du fichier scellé, et le réencoder la rendrait fausse.
 */
export async function GET(
  _: Request,
  { params }: { params: Promise<{ requestId: string }> },
) {
  const { requestId } = await params;
  const upstream = await apiRaw(`/v1/learn/documents/${encodeURIComponent(requestId)}`);

  if (!upstream.ok) {
    const body = (await upstream.json().catch(() => ({}))) as { error?: string };
    return NextResponse.json(
      { error: body.error ?? `l'API a répondu ${upstream.status}` },
      { status: upstream.status },
    );
  }

  return new NextResponse(upstream.body, {
    headers: {
      "Content-Type": "application/pdf",
      "Content-Disposition":
        upstream.headers.get("Content-Disposition") ?? 'attachment; filename="document.pdf"',
      "Cache-Control": "no-store",
    },
  });
}
