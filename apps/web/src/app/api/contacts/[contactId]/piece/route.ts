import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/**
 * Relais de la pièce d'identité.
 *
 * Le fichier ne passe jamais par ici : POST réserve une URL de dépôt direct
 * sur le compartiment chiffré, PUT enregistre la pièce sur la fiche, GET
 * délivre un lien de lecture d'une minute, DELETE l'efface avant l'échéance
 * automatique.
 */
async function relay(path: string, init?: RequestInit) {
  try {
    return NextResponse.json(await apiFetch<unknown>(path, init));
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}

type Params = { params: Promise<{ contactId: string }> };

export async function POST(request: Request, { params }: Params) {
  const { contactId } = await params;
  return relay(`/v1/contacts/${contactId}/piece-identite`, {
    method: "POST",
    body: await request.text(),
  });
}

export async function PUT(request: Request, { params }: Params) {
  const { contactId } = await params;
  return relay(`/v1/contacts/${contactId}/piece-identite`, {
    method: "PUT",
    body: await request.text(),
  });
}

export async function GET(_: Request, { params }: Params) {
  const { contactId } = await params;
  return relay(`/v1/contacts/${contactId}/piece-identite`);
}

export async function DELETE(_: Request, { params }: Params) {
  const { contactId } = await params;
  return relay(`/v1/contacts/${contactId}/piece-identite`, { method: "DELETE" });
}
