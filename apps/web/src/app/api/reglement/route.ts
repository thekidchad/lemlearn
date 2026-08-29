import { apiRaw } from "@/lib/api";

/**
 * Relais du règlement intérieur.
 *
 * Le PDF est renvoyé tel quel plutôt que recopié : il fait quelques dizaines de
 * kilo-octets, mais le faire transiter par un tampon mémoire n'apporterait
 * rien.
 */
export async function GET() {
  const upstream = await apiRaw("/v1/organisme/reglement");
  return new Response(upstream.body, {
    status: upstream.status,
    headers: {
      "Content-Type": upstream.headers.get("Content-Type") ?? "application/pdf",
      "Content-Disposition": 'inline; filename="reglement-interieur.pdf"',
    },
  });
}
