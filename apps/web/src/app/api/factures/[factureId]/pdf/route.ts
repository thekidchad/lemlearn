import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

/**
 * La facture composée.
 *
 * Un brouillon se compose aussi : l'émission étant irréversible, s'en priver
 * reviendrait à demander de signer sans lire.
 */
export async function GET(
  _: Request,
  { params }: { params: Promise<{ factureId: string }> },
) {
  const { factureId } = await params;
  const upstream = await apiRaw(`/v1/factures/${encodeURIComponent(factureId)}/pdf`);
  if (!upstream.ok) {
    const body = (await upstream.json().catch(() => ({}))) as { error?: string };
    return NextResponse.json({ error: body.error ?? "composition refusée" }, {
      status: upstream.status,
    });
  }
  return new NextResponse(upstream.body, {
    headers: {
      "Content-Type": "application/pdf",
      "Content-Disposition":
        upstream.headers.get("Content-Disposition") ?? 'inline; filename="facture.pdf"',
      "Cache-Control": "no-store",
    },
  });
}
