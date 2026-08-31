import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

/**
 * L'archive d'un dossier chez un organisme client, sortie par l'équipe.
 *
 * En GET, parce que le bouton est un lien : le navigateur enregistre le
 * fichier lui-même, sans faire transiter plusieurs mégaoctets par la mémoire
 * de la page. L'archive est retransmise telle quelle — la réencoder
 * invaliderait les empreintes que porte son manifeste.
 */
export async function GET(
  _: Request,
  { params }: { params: Promise<{ orgId: string; fileId: string }> },
) {
  const { orgId, fileId } = await params;
  const upstream = await apiRaw(
    `/v1/admin/orgs/${encodeURIComponent(orgId)}/dossiers/${encodeURIComponent(fileId)}/export`,
    { method: "POST" },
  );

  if (!upstream.ok) {
    const body = (await upstream.json().catch(() => ({}))) as { error?: string };
    return NextResponse.json(
      { error: body.error ?? `l'API a répondu ${upstream.status}` },
      { status: upstream.status },
    );
  }

  return new NextResponse(upstream.body, {
    headers: {
      "Content-Type": "application/zip",
      "Content-Disposition":
        upstream.headers.get("Content-Disposition") ?? 'attachment; filename="dossier.zip"',
      "X-Lemlearn-Pieces": upstream.headers.get("X-Lemlearn-Pieces") ?? "",
      "X-Lemlearn-Missing": upstream.headers.get("X-Lemlearn-Missing") ?? "",
      "Cache-Control": "no-store",
    },
  });
}
