import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

/** Suppression d'une note. */
export async function DELETE(
  _: Request,
  { params }: { params: Promise<{ contactId: string; noteId: string }> },
) {
  const { contactId, noteId } = await params;
  const upstream = await apiRaw(
    `/v1/contacts/${encodeURIComponent(contactId)}/notes/${encodeURIComponent(noteId)}`,
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
