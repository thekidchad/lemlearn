import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/**
 * Relais de la marque d'un organisme, pour l'équipe lemlearn.
 *
 * Le même formulaire sert ici et dans l'espace de l'organisme : seule
 * l'adresse change. L'autorisation, elle, est vérifiée par l'API — cette route
 * ne fait que porter le cookie de session.
 */
export async function PUT(request: Request, { params }: { params: Promise<{ orgId: string }> }) {
  const { orgId } = await params;
  try {
    return NextResponse.json(
      await apiFetch<unknown>(`/v1/admin/orgs/${orgId}/marque`, {
        method: "PUT",
        body: await request.text(),
      }),
    );
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}
