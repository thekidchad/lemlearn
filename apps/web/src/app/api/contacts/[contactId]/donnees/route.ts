import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

/**
 * Portabilité RGPD : les données de la personne, en JSON, téléchargeables.
 *
 * L'API les rend déjà complètes ; ce relais ne fait qu'ajouter le nom de
 * fichier, pour qu'un agent puisse les transmettre sans copier-coller.
 */
export async function GET(_: Request, { params }: { params: Promise<{ contactId: string }> }) {
  const { contactId } = await params;
  const upstream = await apiRaw(`/v1/contacts/${encodeURIComponent(contactId)}/donnees`);

  if (!upstream.ok) {
    const body = (await upstream.json().catch(() => ({}))) as { error?: string };
    return NextResponse.json(
      { error: body.error ?? `l'API a répondu ${upstream.status}` },
      { status: upstream.status },
    );
  }

  return new NextResponse(upstream.body, {
    headers: {
      "Content-Type": "application/json; charset=utf-8",
      "Content-Disposition": `attachment; filename="donnees-${contactId}.json"`,
      "Cache-Control": "no-store",
    },
  });
}
