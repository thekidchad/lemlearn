import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/**
 * Relais de la recherche de la palette.
 *
 * La palette interroge à chaque frappe depuis le navigateur : elle ne peut pas
 * porter le cookie de session vers l'API. L'autorisation reste vérifiée par
 * l'API — cette route ne fait que transporter.
 */
export async function GET(request: Request) {
  const q = new URL(request.url).searchParams.get("q") ?? "";
  try {
    return NextResponse.json(
      await apiFetch<unknown>(`/v1/admin/recherche?q=${encodeURIComponent(q)}`),
    );
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}
