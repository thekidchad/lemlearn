import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/** Notes, rappels et pièces d'une fiche, en un seul appel. */
export async function GET(
  _: Request,
  { params }: { params: Promise<{ contactId: string }> },
) {
  const { contactId } = await params;
  try {
    return NextResponse.json(
      await apiFetch<unknown>(`/v1/contacts/${encodeURIComponent(contactId)}/suivi`),
    );
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}
