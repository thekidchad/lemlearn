import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/**
 * Relais de l'identité de l'organisme.
 *
 * Le formulaire est un composant client — il faut un aperçu immédiat — et ne
 * peut donc pas porter le cookie de session vers l'API. Ce relais le fait pour
 * lui, sans que le jeton descende jamais dans le navigateur.
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

export async function PUT(request: Request) {
  return relay("/v1/marque", { method: "PUT", body: await request.text() });
}
