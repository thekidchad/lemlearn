import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

/**
 * Attribuer une formation à un stagiaire.
 *
 * Le serveur assemble le geste : il reprend ou ouvre la session, ouvre le
 * dossier, puis inscrit. L'écran n'a qu'à dire quelle formation.
 */
export async function POST(
  request: Request,
  { params }: { params: Promise<{ contactId: string }> },
) {
  const { contactId } = await params;
  const upstream = await apiRaw(
    `/v1/contacts/${encodeURIComponent(contactId)}/inscription`,
    {
      method: "POST",
      body: await request.text(),
      headers: { "Content-Type": "application/json" },
    },
  );
  const payload = (await upstream.json().catch(() => ({}))) as Record<string, unknown>;
  return NextResponse.json(payload, { status: upstream.status });
}
