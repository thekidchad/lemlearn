import { NextResponse } from "next/server";
import { API_COOKIE, SESSION_COOKIE, apiRaw } from "@/lib/api";

/**
 * Acceptation d'une invitation.
 *
 * L'API ouvre la session et pose son cookie sur son domaine ; l'application
 * repose le sien, comme à la connexion — sans quoi l'apprenant se retrouverait
 * authentifié côté API et déconnecté à l'écran.
 */
export async function POST(
  request: Request,
  { params }: { params: Promise<{ token: string }> },
) {
  const { token } = await params;
  const upstream = await apiRaw(`/v1/invitation/${encodeURIComponent(token)}`, {
    method: "POST",
    body: await request.text(),
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
  const token_ = upstream.headers
    .getSetCookie()
    .map((cookie) => cookie.split(";")[0])
    .find((pair) => pair.startsWith(`${API_COOKIE}=`))
    ?.slice(API_COOKIE.length + 1);

  if (!token_) {
    return NextResponse.json(
      { error: "le compte est ouvert mais la session n'a pas pu être posée" },
      { status: 502 },
    );
  }

  response.cookies.set(SESSION_COOKIE, token_, {
    httpOnly: true,
    sameSite: "lax",
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge: 12 * 60 * 60,
  });
  return response;
}
