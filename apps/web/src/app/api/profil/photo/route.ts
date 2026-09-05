import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

/**
 * La photo de profil.
 *
 * POST signe le dépôt, PUT rattache la photo déposée — ou la retire quand la
 * clé est vide. Le fichier ne passe jamais par ici.
 */
export async function POST(request: Request) {
  return relayer("POST", await request.text());
}

export async function PUT(request: Request) {
  return relayer("PUT", await request.text());
}

async function relayer(method: string, body: string) {
  const upstream = await apiRaw("/v1/profil/photo", {
    method,
    body,
    headers: { "Content-Type": "application/json" },
  });
  const payload = (await upstream.json().catch(() => ({}))) as Record<string, unknown>;
  return NextResponse.json(payload, { status: upstream.status });
}
