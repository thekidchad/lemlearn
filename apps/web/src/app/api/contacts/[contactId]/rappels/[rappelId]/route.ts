import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

type Params = { params: Promise<{ contactId: string; rappelId: string }> };

/** Un rappel se ferme, se rouvre ou se supprime. */
export async function PATCH(request: Request, { params }: Params) {
  const { contactId, rappelId } = await params;
  const upstream = await apiRaw(
    `/v1/contacts/${encodeURIComponent(contactId)}/rappels/${encodeURIComponent(rappelId)}`,
    {
      method: "PATCH",
      body: await request.text(),
      headers: { "Content-Type": "application/json" },
    },
  );
  const payload = (await upstream.json().catch(() => ({}))) as Record<string, unknown>;
  return NextResponse.json(payload, { status: upstream.status });
}

export async function DELETE(_: Request, { params }: Params) {
  const { contactId, rappelId } = await params;
  const upstream = await apiRaw(
    `/v1/contacts/${encodeURIComponent(contactId)}/rappels/${encodeURIComponent(rappelId)}`,
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
