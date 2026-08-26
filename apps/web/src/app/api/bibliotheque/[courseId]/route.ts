import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/** Import d'une formation de la bibliothèque dans le catalogue de l'organisme. */
export async function POST(_: Request, { params }: { params: Promise<{ courseId: string }> }) {
  const { courseId } = await params;
  try {
    return NextResponse.json(
      await apiFetch<unknown>(`/v1/bibliotheque/${encodeURIComponent(courseId)}/importer`, {
        method: "POST",
      }),
    );
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}
