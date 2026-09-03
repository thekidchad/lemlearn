import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

type Params = { params: Promise<{ contactId: string; pieceId: string }> };

/** Lien de lecture d'une pièce, ou suppression. */
export async function GET(_: Request, { params }: Params) {
  const { contactId, pieceId } = await params;
  const upstream = await apiRaw(
    `/v1/contacts/${encodeURIComponent(contactId)}/pieces/${encodeURIComponent(pieceId)}`,
  );
  const payload = (await upstream.json().catch(() => ({}))) as Record<string, unknown>;
  return NextResponse.json(payload, { status: upstream.status });
}

export async function DELETE(_: Request, { params }: Params) {
  const { contactId, pieceId } = await params;
  const upstream = await apiRaw(
    `/v1/contacts/${encodeURIComponent(contactId)}/pieces/${encodeURIComponent(pieceId)}`,
    { method: "DELETE" },
  );
  if (!upstream.ok) {
    const body = (await upstream.json().catch(() => ({}))) as { error?: string };
    return NextResponse.json({ error: body.error ?? "suppression refusée" }, {
      status: upstream.status,
    });
  }
  return new NextResponse(null, { status: 204 });
}
