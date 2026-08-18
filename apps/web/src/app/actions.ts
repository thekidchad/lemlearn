"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { API_COOKIE, SESSION_COOKIE } from "@/lib/api";

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

  let response: Response;
  try {
    response = await fetch(`${API_URL}/v1/auth/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, password }),
      cache: "no-store",
    });
  } catch (error) {
    // Une panne réseau doit se lire à l'écran, pas seulement dans la console
    // du serveur : sans cela, le formulaire reste muet et l'utilisateur ne
    // sait pas s'il a mal saisi son mot de passe ou si le service est absent.
    const cause = error instanceof Error ? (error.cause as Error | undefined) : undefined;
    return {
      error: `Service d'authentification injoignable (${API_URL}) : ${cause?.message ?? (error as Error).message}`,
    };
  }

  if (!response.ok) {
    const body = (await response.json().catch(() => ({}))) as { error?: string };
    return { error: body.error ?? "Connexion impossible." };
  }

  // Le cookie arrive dans Set-Cookie ; on le recopie sur notre domaine avec
  // les mêmes garanties : httpOnly, SameSite, et Secure hors développement.
  const raw = response.headers.get("set-cookie");
  // Le nom vient de l'API, qui le choisit selon son propre environnement.
  const token = raw?.match(new RegExp(`(?:^|[;, ])${API_COOKIE}=([^;]+)`))?.[1];
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
      headers: { Cookie: `${API_COOKIE}=${session.value}` },
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
