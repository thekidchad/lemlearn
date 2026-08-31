import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/** Relais de la fiche d'une personne chez un organisme client. */
export async function GET(
  _: Request,
  { params }: { params: Promise<{ orgId: string; contactId: string }> },
) {
  const { orgId, contactId } = await params;
  try {
    return NextResponse.json(
      await apiFetch<unknown>(
        `/v1/admin/orgs/${encodeURIComponent(orgId)}/contacts/${encodeURIComponent(contactId)}`,
      ),
    );
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}
