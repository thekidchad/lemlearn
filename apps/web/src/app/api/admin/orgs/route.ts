import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

/**
 * Ouverture de l'espace d'un organisme client.
 *
 * La réponse est retransmise telle quelle, y compris quand l'API répond 202 :
 * ce code dit « l'espace existe, mais le courriel n'est pas parti », et le lien
 * d'invitation qu'il porte est alors la seule façon de rattraper — recommencer
 * l'ouverture échouerait, l'adresse étant désormais réservée.
 */
export async function POST(request: Request) {
  const body = await request.text();
  const upstream = await apiRaw("/v1/admin/orgs", {
    method: "POST",
    body: body || "{}",
    headers: { "Content-Type": "application/json" },
  });

  const payload = (await upstream.json().catch(() => ({}))) as { error?: string };
  if (!upstream.ok) {
    return NextResponse.json(
      { error: payload.error ?? `l'API a répondu ${upstream.status}` },
      { status: upstream.status },
    );
  }
  return NextResponse.json(payload, { status: upstream.status });
}
