import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

/** Les trois actes d'une facture : l'émettre, l'encaisser, l'annuler par un avoir. */
const ACTIONS = new Set(["emission", "paiement", "avoir"]);

export async function POST(
  request: Request,
  { params }: { params: Promise<{ factureId: string; action: string }> },
) {
  const { factureId, action } = await params;
  if (!ACTIONS.has(action)) {
    return NextResponse.json({ error: "action inconnue" }, { status: 404 });
  }
  const corps = await request.text();
  const upstream = await apiRaw(
    `/v1/factures/${encodeURIComponent(factureId)}/${action}`,
    { method: "POST", body: corps || "{}", headers: { "Content-Type": "application/json" } },
  );
  const payload = (await upstream.json().catch(() => ({}))) as Record<string, unknown>;
  return NextResponse.json(payload, { status: upstream.status });
}
