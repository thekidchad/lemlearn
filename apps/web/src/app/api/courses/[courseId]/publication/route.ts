import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

/**
 * Publication d'une formation.
 *
 * Le corps de la réponse est retransmis même en cas de refus : l'API répond 409
 * avec la liste des mentions manquantes, et c'est cette liste que l'écran doit
 * montrer — « publication refusée » tout court n'aide personne.
 */
export async function PUT(
  request: Request,
  { params }: { params: Promise<{ courseId: string }> },
) {
  const { courseId } = await params;
  const upstream = await apiRaw(`/v1/courses/${encodeURIComponent(courseId)}/publication`, {
    method: "PUT",
    body: await request.text(),
    headers: { "Content-Type": "application/json" },
  });
  const payload = (await upstream.json().catch(() => ({}))) as Record<string, unknown>;
  return NextResponse.json(payload, { status: upstream.status });
}
