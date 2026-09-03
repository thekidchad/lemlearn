import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

type Params = { params: Promise<{ factureId: string }> };

/** Une facture : lecture, correction d'un brouillon, suppression. */
export async function GET(_: Request, { params }: Params) {
  const { factureId } = await params;
  return relayer(`/v1/factures/${encodeURIComponent(factureId)}`, "GET");
}

export async function PATCH(request: Request, { params }: Params) {
  const { factureId } = await params;
  return relayer(`/v1/factures/${encodeURIComponent(factureId)}`, "PATCH", await request.text());
}

export async function DELETE(_: Request, { params }: Params) {
  const { factureId } = await params;
  const upstream = await apiRaw(`/v1/factures/${encodeURIComponent(factureId)}`, {
    method: "DELETE",
  });
  if (!upstream.ok) {
    const body = (await upstream.json().catch(() => ({}))) as { error?: string };
    return NextResponse.json({ error: body.error ?? "suppression refusée" }, {
      status: upstream.status,
    });
  }
  return new NextResponse(null, { status: 204 });
}

async function relayer(path: string, method: string, body?: string) {
  const upstream = await apiRaw(path, {
    method,
    body,
    headers: { "Content-Type": "application/json" },
  });
  const payload = (await upstream.json().catch(() => ({}))) as Record<string, unknown>;
  return NextResponse.json(payload, { status: upstream.status });
}
