import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

/** Ouverture d'un accès à un collaborateur. */
export async function POST(request: Request) {
  const upstream = await apiRaw("/v1/equipe", {
    method: "POST",
    body: await request.text(),
    headers: { "Content-Type": "application/json" },
  });
  const payload = (await upstream.json().catch(() => ({}))) as Record<string, unknown>;
  return NextResponse.json(payload, { status: upstream.status });
}
