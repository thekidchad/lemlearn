import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/** Relais de l'identité juridique de l'organisme. */
export async function PUT(request: Request) {
  try {
    return NextResponse.json(
      await apiFetch<unknown>("/v1/organisme", {
        method: "PUT",
        body: await request.text(),
      }),
    );
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}
