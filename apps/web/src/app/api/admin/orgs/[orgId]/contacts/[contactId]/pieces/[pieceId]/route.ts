import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/** Lien de lecture d'une pièce jointe, depuis la console de l'équipe. */
export async function GET(
  _: Request,
  { params }: { params: Promise<{ orgId: string; contactId: string; pieceId: string }> },
) {
  const { orgId, contactId, pieceId } = await params;
  try {
    return NextResponse.json(
      await apiFetch<unknown>(
        `/v1/admin/orgs/${encodeURIComponent(orgId)}/contacts/${encodeURIComponent(contactId)}` +
          `/pieces/${encodeURIComponent(pieceId)}`,
      ),
    );
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}
