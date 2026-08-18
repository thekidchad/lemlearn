import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

/**
 * Relais de l'export d'un dossier.
 *
 * L'archive est retransmise telle quelle, sans passer par la mémoire du
 * serveur : un dossier complet pèse plusieurs mégaoctets, et son empreinte
 * figure au manifeste — la réencoder invaliderait ce que le manifeste
 * annonce.
 */
export async function POST(
  _: Request,
  { params }: { params: Promise<{ fileId: string }> },
) {
  const { fileId } = await params;
  const upstream = await apiRaw(`/v1/files/${encodeURIComponent(fileId)}/export`, {
    method: "POST",
  });

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
      // Le nombre de pièces et de manques voyage en en-tête : le bouton peut
      // le dire sans ouvrir l'archive.
      "X-Lemlearn-Pieces": upstream.headers.get("X-Lemlearn-Pieces") ?? "",
      "X-Lemlearn-Missing": upstream.headers.get("X-Lemlearn-Missing") ?? "",
      "Cache-Control": "no-store",
    },
  });
}
