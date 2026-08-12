"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { SESSION_COOKIE } from "@/lib/api";

const API_URL = process.env.LEMLEARN_API_URL ?? "http://localhost:8787";

/**
 * signIn ouvre une session.
 *
 * L'appel passe par le serveur Next plutôt que par le navigateur : le cookie
 * posé par l'API est ainsi recopié sur notre propre domaine, et le jeton
 * n'est jamais manipulé par du JavaScript client.
 */
export async function signIn(_: { error?: string } | undefined, form: FormData) {
  const email = String(form.get("email") ?? "");
  const password = String(form.get("password") ?? "");

  const response = await fetch(`${API_URL}/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
    cache: "no-store",
  });

  if (!response.ok) {
    const body = (await response.json().catch(() => ({}))) as { error?: string };
    return { error: body.error ?? "Connexion impossible." };
  }

  // Le cookie arrive dans Set-Cookie ; on le recopie sur notre domaine avec
  // les mêmes garanties : httpOnly, SameSite, et Secure hors développement.
  const raw = response.headers.get("set-cookie");
  const token = raw?.match(new RegExp(`${SESSION_COOKIE}=([^;]+)`))?.[1];
  if (!token) {
    return { error: "Le service d'authentification n'a pas renvoyé de session." };
  }

  const store = await cookies();
  store.set(SESSION_COOKIE, token, {
    httpOnly: true,
    sameSite: "lax",
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge: 12 * 60 * 60,
  });

  redirect("/pipeline");
}

/** signOut révoque la session côté API puis efface le cookie. */
export async function signOut() {
  const store = await cookies();
  const session = store.get(SESSION_COOKIE);

  if (session) {
    await fetch(`${API_URL}/v1/auth/logout`, {
      method: "POST",
      headers: { Cookie: `${SESSION_COOKIE}=${session.value}` },
      cache: "no-store",
    }).catch(() => {
      // La révocation côté serveur a échoué : on efface tout de même le
      // cookie local, sinon l'utilisateur resterait bloqué sur une session
      // qu'il croit fermée.
    });
  }

  store.delete(SESSION_COOKIE);
  redirect("/connexion");
}
