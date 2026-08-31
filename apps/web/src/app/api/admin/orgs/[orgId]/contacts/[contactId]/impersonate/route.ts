import { NextResponse } from "next/server";
import { API_COOKIE, SESSION_COOKIE, apiRaw } from "@/lib/api";

/**
 * Entrer dans le compte d'une personne depuis la vue de l'équipe.
 *
 * L'API ouvre la session et pose son cookie sur son propre domaine ; sans
 * cette recopie, l'application resterait sur l'ancienne session et l'écran
 * ne bougerait pas.
 */
export async function POST(
  _: Request,
  { params }: { params: Promise<{ orgId: string; contactId: string }> },
) {
  const { orgId, contactId } = await params;
  const upstream = await apiRaw(
    `/v1/admin/orgs/${encodeURIComponent(orgId)}/contacts/${encodeURIComponent(contactId)}/impersonate`,
    { method: "POST" },
  );

  const payload = (await upstream.json().catch(() => ({}))) as {
    error?: string;
    landing?: string;
  };
  if (!upstream.ok) {
    return NextResponse.json(
      { error: payload.error ?? `l'API a répondu ${upstream.status}` },
      { status: upstream.status },
    );
  }

  const token = jetonDe(upstream.headers.getSetCookie());
  if (!token) {
    return NextResponse.json(
      { error: "l'API n'a pas ouvert de session : rien n'a changé" },
      { status: 502 },
    );
  }

  const response = NextResponse.json(payload);
  response.cookies.set(SESSION_COOKIE, token, {
    httpOnly: true,
    sameSite: "lax",
    secure: process.env.NODE_ENV === "production",
    path: "/",
  });
  return response;
}

/** jetonDe extrait le jeton de session de l'en-tête de l'API. */
function jetonDe(cookies: string[]): string | null {
  for (const cookie of cookies) {
    const [pair] = cookie.split(";");
    const separator = pair.indexOf("=");
    if (separator > 0 && pair.slice(0, separator).trim() === API_COOKIE) {
      return pair.slice(separator + 1);
    }
  }
  return null;
}
