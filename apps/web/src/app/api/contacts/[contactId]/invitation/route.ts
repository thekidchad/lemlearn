import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/** Ouverture d'un accès à l'espace apprenant. */
export async function POST(
  _: Request,
  { params }: { params: Promise<{ contactId: string }> },
) {
  const { contactId } = await params;
  try {
    return NextResponse.json(
      await apiFetch<unknown>(`/v1/contacts/${contactId}/invitation`, { method: "POST" }),
    );
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}
