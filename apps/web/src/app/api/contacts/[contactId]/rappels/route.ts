import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

/** Pose d'un rappel sur une fiche. */
export async function POST(
  request: Request,
  { params }: { params: Promise<{ contactId: string }> },
) {
  const { contactId } = await params;
  const upstream = await apiRaw(`/v1/contacts/${encodeURIComponent(contactId)}/rappels`, {
    method: "POST",
    body: await request.text(),
    headers: { "Content-Type": "application/json" },
  });
  const payload = (await upstream.json().catch(() => ({}))) as Record<string, unknown>;
  return NextResponse.json(payload, { status: upstream.status });
}
