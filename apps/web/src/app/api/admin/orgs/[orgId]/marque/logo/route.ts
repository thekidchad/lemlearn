import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/** Réservation d'un dépôt de logo pour un organisme donné. */
export async function POST(request: Request, { params }: { params: Promise<{ orgId: string }> }) {
  const { orgId } = await params;
  try {
    return NextResponse.json(
      await apiFetch<unknown>(`/v1/admin/orgs/${orgId}/marque/logo`, {
        method: "POST",
        body: await request.text(),
      }),
    );
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}
