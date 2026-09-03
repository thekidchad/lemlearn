import { NextResponse } from "next/server";
import { apiFetch, apiRaw, ApiError } from "@/lib/api";

/** Les factures de l'organisme. */
export async function GET() {
  try {
    return NextResponse.json(await apiFetch<unknown>("/v1/factures"));
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}

export async function POST(request: Request) {
  const upstream = await apiRaw("/v1/factures", {
    method: "POST",
    body: await request.text(),
    headers: { "Content-Type": "application/json" },
  });
  const payload = (await upstream.json().catch(() => ({}))) as Record<string, unknown>;
  return NextResponse.json(payload, { status: upstream.status });
}
