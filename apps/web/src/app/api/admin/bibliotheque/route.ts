import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/** Relais de la bibliothèque de formations, côté équipe. */
async function relay(path: string, init?: RequestInit) {
  try {
    return NextResponse.json(await apiFetch<unknown>(path, init));
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}

export async function POST(request: Request) {
  const id = new URL(request.url).searchParams.get("id");
  const body = await request.text();
  return id
    ? relay(`/v1/admin/bibliotheque/${encodeURIComponent(id)}`, { method: "PUT", body })
    : relay("/v1/admin/bibliotheque", { method: "POST", body });
}

export async function DELETE(request: Request) {
  const id = new URL(request.url).searchParams.get("id");
  if (!id) return NextResponse.json({ error: "formation non précisée" }, { status: 400 });
  return relay(`/v1/admin/bibliotheque/${encodeURIComponent(id)}`, { method: "DELETE" });
}
