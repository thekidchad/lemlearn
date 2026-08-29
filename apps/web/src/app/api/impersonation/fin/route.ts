import { NextResponse } from "next/server";
import { SESSION_COOKIE, API_COOKIE } from "@/lib/api";
import { cookies } from "next/headers";

/**
 * Relais de la sortie d'impersonation.
 *
 * L'API répond avec un nouveau cookie de session ; il faut le recopier sur
 * notre domaine, comme le fait la connexion. Sans cette recopie, le navigateur
 * garderait la session du client et le retour n'aurait servi à rien.
 */
export async function POST() {
  try {
    const store = await cookies();
    const session = store.get(SESSION_COOKIE);
    const response = await fetch(`${process.env.LEMLEARN_API_URL}/v1/impersonation/fin`, {
      method: "POST",
      cache: "no-store",
      headers: session ? { Cookie: `${API_COOKIE}=${session.value}` } : {},
    });

    if (!response.ok) {
      const body = (await response.json().catch(() => ({}))) as { error?: string };
      return NextResponse.json(
        { error: body.error ?? "retour impossible" },
        { status: response.status },
      );
    }

    const raw = response.headers.get("set-cookie");
    const token = raw?.match(new RegExp(`(?:^|[;, ])${API_COOKIE}=([^;]+)`))?.[1];
    if (token) {
      store.set(SESSION_COOKIE, token, {
        httpOnly: true,
        sameSite: "lax",
        secure: process.env.NODE_ENV === "production",
        path: "/",
        maxAge: 12 * 60 * 60,
      });
    }
    return NextResponse.json(await response.json());
  } catch {
    return NextResponse.json({ error: "retour impossible" }, { status: 500 });
  }
}
