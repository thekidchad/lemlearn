import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/**
 * Relais du visuel d'une formation.
 *
 * POST réserve une URL de dépôt direct vers le compartiment public, PUT
 * rattache le fichier déposé — ou le retire quand la clé est vide.
 */
async function relay(path: string, init?: RequestInit) {
  try {
    return NextResponse.json(await apiFetch<unknown>(path, init));
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}

type Params = { params: Promise<{ courseId: string }> };

export async function POST(request: Request, { params }: Params) {
  const { courseId } = await params;
  return relay(`/v1/courses/${courseId}/visuel`, {
    method: "POST",
    body: await request.text(),
  });
}

export async function PUT(request: Request, { params }: Params) {
  const { courseId } = await params;
  return relay(`/v1/courses/${courseId}/visuel`, {
    method: "PUT",
    body: await request.text(),
  });
}
