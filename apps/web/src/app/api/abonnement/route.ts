import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/** Relais de l'ouverture d'un paiement. */
export async function POST(request: Request) {
  try {
    const result = await apiFetch<unknown>("/v1/abonnement/paiement", {
      method: "POST",
      body: await request.text(),
    });
    return NextResponse.json(result);
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}
