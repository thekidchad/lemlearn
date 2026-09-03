import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

/**
 * Reprise d'un portefeuille depuis un tableur.
 *
 * Le fichier passe en corps brut : il n'y en a qu'un, et un multipart
 * obligerait à assembler des frontières pour rien.
 */
export async function POST(request: Request) {
  const kind = new URL(request.url).searchParams.get("kind") ?? "learner";
  const upstream = await apiRaw(`/v1/contacts/import?kind=${encodeURIComponent(kind)}`, {
    method: "POST",
    body: await request.text(),
    headers: { "Content-Type": "text/csv" },
  });
  const payload = (await upstream.json().catch(() => ({}))) as Record<string, unknown>;
  return NextResponse.json(payload, { status: upstream.status });
}
