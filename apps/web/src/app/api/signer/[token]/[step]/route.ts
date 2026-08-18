import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

/**
 * Relais du parcours de signature.
 *
 * Le signataire n'a pas de compte : ces routes de l'API sont publiques, leur
 * légitimité venant du jeton du lien puis du code envoyé par courriel. Passer
 * par le serveur de l'application évite d'ouvrir le CORS de l'API à un
 * navigateur, et garde une seule origine dans la barre d'adresse pendant qu'on
 * demande à quelqu'un de signer un contrat.
 */
const STEPS = new Set(["document", "otp", "confirm", "sealed"]);

async function relay(
  request: Request,
  params: Promise<{ token: string; step: string }>,
  method: "GET" | "POST",
) {
  const { token, step } = await params;
  if (!STEPS.has(step)) {
    return NextResponse.json({ error: "étape inconnue" }, { status: 404 });
  }

  const response = await apiRaw(`/v1/sign/${encodeURIComponent(token)}/${step}`, {
    method,
    ...(method === "POST"
      ? { body: await request.text(), headers: { "Content-Type": "application/json" } }
      : {}),
  });

  // Le corps est retransmis tel quel : un PDF scellé ne doit pas être réencodé
  // en chemin, son empreinte étant précisément ce que le signataire vérifie.
  return new NextResponse(response.body, {
    status: response.status,
    headers: {
      "Content-Type": response.headers.get("Content-Type") ?? "application/json",
      "Cache-Control": "no-store",
      ...(response.headers.get("Content-Disposition")
        ? { "Content-Disposition": response.headers.get("Content-Disposition")! }
        : {}),
    },
  });
}

export async function GET(
  request: Request,
  { params }: { params: Promise<{ token: string; step: string }> },
) {
  return relay(request, params, "GET");
}

export async function POST(
  request: Request,
  { params }: { params: Promise<{ token: string; step: string }> },
) {
  return relay(request, params, "POST");
}
