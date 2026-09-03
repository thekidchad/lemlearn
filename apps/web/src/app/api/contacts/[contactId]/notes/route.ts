import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

/** Ajout d'une note sur une fiche. */
export async function POST(
  request: Request,
  { params }: { params: Promise<{ contactId: string }> },
) {
  const { contactId } = await params;
  const upstream = await apiRaw(`/v1/contacts/${encodeURIComponent(contactId)}/notes`, {
    method: "POST",
    body: await request.text(),
    headers: { "Content-Type": "application/json" },
  });
  const payload = (await upstream.json().catch(() => ({}))) as Record<string, unknown>;
  return NextResponse.json(payload, { status: upstream.status });
}
