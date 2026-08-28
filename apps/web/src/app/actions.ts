"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { API_COOKIE, SESSION_COOKIE } from "@/lib/api";
import { THEME_COOKIE } from "@/lib/theme";

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
  const body = (await response.json().catch(() => ({}))) as {
    user?: { role?: string };
    brand?: { theme?: string };
  };
  const role = body.user?.role;

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

  // Le thème par défaut de l'organisme, posé à la connexion et pas plus tard.
  //
  // C'est le seul moment où on peut le faire sans clignotement : la coque est
  // rendue sous la racine du document, et seule la racine porte le thème. On
  // ne l'écrit que si la personne n'a rien choisi — un réglage explicite ne se
  // fait pas écraser par une préférence d'organisme.
  const theme = body.brand?.theme;
  if (!store.get(THEME_COOKIE) && (theme === "light" || theme === "dark")) {
    store.set(THEME_COOKIE, theme, {
      sameSite: "lax",
      secure: process.env.NODE_ENV === "production",
      path: "/",
      maxAge: 365 * 24 * 60 * 60,
    });
  }

  // Le rôle décide de la porte d'entrée : un apprenant n'a rien à faire sur
  // le pipeline — il y recevrait un 403 — et l'équipe lemlearn arrive sur ses
  // clients, pas sur le pipeline vide de sa propre organisation.
  redirect(
    role === "learner" ? "/apprenant" : role === "superadmin" ? "/admin" : "/pipeline",
  );
}

/**
 * signUp crée l'organisation et son compte propriétaire, puis connecte.
 *
 * L'inscription ne connecte pas d'elle-même côté API : on enchaîne sur la
 * connexion, pour que le chemin qui servira tous les jours soit exercé dès la
 * première minute plutôt que découvert au retour de vacances.
 */
export async function signUp(_: { error?: string } | undefined, form: FormData) {
  const payload = {
    orgName: String(form.get("orgName") ?? "").trim(),
    email: String(form.get("email") ?? "").trim(),
    password: String(form.get("password") ?? ""),
    firstName: String(form.get("firstName") ?? "").trim(),
    lastName: String(form.get("lastName") ?? "").trim(),
  };

  let response: Response;
  try {
    response = await fetch(`${API_URL}/v1/auth/register`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
      cache: "no-store",
    });
  } catch (error) {
    const cause = error instanceof Error ? (error.cause as Error | undefined) : undefined;
    return {
      error: `Service d'inscription injoignable (${API_URL}) : ${cause?.message ?? (error as Error).message}`,
    };
  }

  if (!response.ok) {
    const body = (await response.json().catch(() => ({}))) as { error?: string };
    return { error: body.error ?? "Inscription impossible." };
  }

  const credentials = new FormData();
  credentials.set("email", payload.email);
  credentials.set("password", payload.password);
  return signIn(undefined, credentials);
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
