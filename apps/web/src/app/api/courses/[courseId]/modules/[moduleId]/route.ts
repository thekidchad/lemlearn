import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/** Relais de la mise à jour d'un module — attacher une vidéo, un contrôle. */
export async function PATCH(
  request: Request,
  { params }: { params: Promise<{ courseId: string; moduleId: string }> },
) {
  const { courseId, moduleId } = await params;
  try {
    return NextResponse.json(
      await apiFetch<unknown>(`/v1/courses/${courseId}/modules/${moduleId}`, {
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
