import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/** Relais de l'origine des fonds d'un dossier. */
export async function PATCH(
  request: Request,
  { params }: { params: Promise<{ fileId: string }> },
) {
  const { fileId } = await params;
  try {
    return NextResponse.json(
      await apiFetch<unknown>(`/v1/files/${fileId}/financement`, {
        method: "PATCH",
        body: await request.text(),
      }),
    );
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}
