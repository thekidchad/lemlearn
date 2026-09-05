import { NextResponse } from "next/server";
import { apiFetch, apiRaw, ApiError } from "@/lib/api";

/** Le compte de celui qui est connecté. */
export async function GET() {
  try {
    return NextResponse.json(await apiFetch<unknown>("/v1/profil"));
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}

export async function PATCH(request: Request) {
  const upstream = await apiRaw("/v1/profil", {
    method: "PATCH",
    body: await request.text(),
    headers: { "Content-Type": "application/json" },
  });
  const payload = (await upstream.json().catch(() => ({}))) as Record<string, unknown>;
  return NextResponse.json(payload, { status: upstream.status });
}
