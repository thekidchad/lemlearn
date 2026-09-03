import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

/**
 * Modules d'une formation de la bibliothèque.
 *
 * POST crée ou met à jour — le service distingue les deux sur la présence de
 * l'identifiant. DELETE retire, avec le module en paramètre : une adresse
 * imbriquée obligerait à un second fichier pour un seul verbe.
 */
export async function POST(
  request: Request,
  { params }: { params: Promise<{ courseId: string }> },
) {
  const { courseId } = await params;
  const upstream = await apiRaw(
    `/v1/admin/bibliotheque/${encodeURIComponent(courseId)}/modules`,
    {
      method: "POST",
      body: await request.text(),
      headers: { "Content-Type": "application/json" },
    },
  );
  const payload = (await upstream.json().catch(() => ({}))) as Record<string, unknown>;
  return NextResponse.json(payload, { status: upstream.status });
}

export async function DELETE(
  request: Request,
  { params }: { params: Promise<{ courseId: string }> },
) {
  const { courseId } = await params;
  const moduleId = new URL(request.url).searchParams.get("id") ?? "";
  const upstream = await apiRaw(
    `/v1/admin/bibliotheque/${encodeURIComponent(courseId)}/modules/${encodeURIComponent(moduleId)}`,
    { method: "DELETE" },
  );
  if (!upstream.ok) {
    const body = (await upstream.json().catch(() => ({}))) as { error?: string };
    return NextResponse.json(
      { error: body.error ?? `l'API a répondu ${upstream.status}` },
      { status: upstream.status },
    );
  }
  return new NextResponse(null, { status: 204 });
}
