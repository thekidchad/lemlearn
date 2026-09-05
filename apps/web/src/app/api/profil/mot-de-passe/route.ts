import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

/** Changement du mot de passe, l'actuel à l'appui. */
export async function POST(request: Request) {
  const upstream = await apiRaw("/v1/profil/mot-de-passe", {
    method: "POST",
    body: await request.text(),
    headers: { "Content-Type": "application/json" },
  });
  if (!upstream.ok) {
    const body = (await upstream.json().catch(() => ({}))) as { error?: string };
    return NextResponse.json({ error: body.error ?? "changement refusé" }, {
      status: upstream.status,
    });
  }
  return new NextResponse(null, { status: 204 });
}
