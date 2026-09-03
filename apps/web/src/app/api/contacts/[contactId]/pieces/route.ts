import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

type Params = { params: Promise<{ contactId: string }> };

/**
 * Pièces jointes d'une fiche.
 *
 * POST signe le dépôt, PUT enregistre la pièce déposée. Le fichier ne passe
 * jamais par ici : le navigateur écrit directement dans le compartiment.
 */
export async function POST(request: Request, { params }: Params) {
  const { contactId } = await params;
  return relayer(contactId, "POST", await request.text());
}

export async function PUT(request: Request, { params }: Params) {
  const { contactId } = await params;
  return relayer(contactId, "PUT", await request.text());
}

async function relayer(contactId: string, method: string, body: string) {
  const upstream = await apiRaw(`/v1/contacts/${encodeURIComponent(contactId)}/pieces`, {
    method,
    body,
    headers: { "Content-Type": "application/json" },
  });
  const payload = (await upstream.json().catch(() => ({}))) as Record<string, unknown>;
  return NextResponse.json(payload, { status: upstream.status });
}
