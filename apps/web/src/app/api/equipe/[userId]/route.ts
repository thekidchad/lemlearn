import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

/** Changement de rôle, suspension ou rétablissement d'un accès. */
export async function PATCH(
  request: Request,
  { params }: { params: Promise<{ userId: string }> },
) {
  const { userId } = await params;
  const upstream = await apiRaw(`/v1/equipe/${encodeURIComponent(userId)}`, {
    method: "PATCH",
    body: await request.text(),
    headers: { "Content-Type": "application/json" },
  });
  const payload = (await upstream.json().catch(() => ({}))) as Record<string, unknown>;
  return NextResponse.json(payload, { status: upstream.status });
}
