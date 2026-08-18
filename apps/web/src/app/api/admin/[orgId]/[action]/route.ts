import { NextResponse } from "next/server";
import { API_COOKIE, SESSION_COOKIE, apiRaw } from "@/lib/api";

/** Relais des actions de la vue super-admin. */
const ACTIONS = new Set(["plan", "impersonate"]);

export async function POST(
  request: Request,
  { params }: { params: Promise<{ orgId: string; action: string }> },
) {
  const { orgId, action } = await params;
  if (!ACTIONS.has(action)) {
    return NextResponse.json({ error: "action inconnue" }, { status: 404 });
  }

  const body = await request.text();
  const upstream = await apiRaw(`/v1/admin/orgs/${encodeURIComponent(orgId)}/${action}`, {
    method: "POST",
    body: body || "{}",
    headers: { "Content-Type": "application/json" },
  });

  const payload = (await upstream.json()) as { error?: string };
  if (!upstream.ok) {
    return NextResponse.json(
      { error: payload.error ?? `l'API a répondu ${upstream.status}` },
      { status: upstream.status },
    );
  }

  const response = NextResponse.json(payload);

  // L'impersonation ouvre une nouvelle session côté API, qui pose son cookie
  // sur *son* domaine. L'application doit reposer le sien : sans cette
  // recopie, l'API basculerait d'organisation et l'écran continuerait
  // d'afficher l'ancienne.
  if (action === "impersonate") {
    const token = tokenFromCookies(upstream.headers.getSetCookie());
    if (!token) {
      return NextResponse.json(
        { error: "l'API n'a pas ouvert de session : rien n'a changé" },
        { status: 502 },
      );
    }
    response.cookies.set(SESSION_COOKIE, token, {
      httpOnly: true,
      sameSite: "lax",
      secure: process.env.NODE_ENV === "production",
      path: "/",
    });
  }

  return response;
}

/** tokenFromCookies extrait le jeton de session de l'en-tête de l'API. */
function tokenFromCookies(cookies: string[]): string | null {
  for (const cookie of cookies) {
    const [pair] = cookie.split(";");
    const separator = pair.indexOf("=");
    if (separator > 0 && pair.slice(0, separator).trim() === API_COOKIE) {
      return pair.slice(separator + 1);
    }
  }
  return null;
}

