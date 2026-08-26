import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/** Relais de l'édition des gabarits de courriels. */
async function relay(path: string, init?: RequestInit) {
  try {
    return NextResponse.json(await apiFetch<unknown>(path, init));
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}

type Params = { params: Promise<{ key: string }> };

export async function PUT(request: Request, { params }: Params) {
  const { key } = await params;
  return relay(`/v1/admin/gabarits/${encodeURIComponent(key)}`, {
    method: "PUT",
    body: await request.text(),
  });
}

export async function DELETE(_: Request, { params }: Params) {
  const { key } = await params;
  return relay(`/v1/admin/gabarits/${encodeURIComponent(key)}`, { method: "DELETE" });
}
