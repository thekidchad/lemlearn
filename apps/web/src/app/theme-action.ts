"use server";

import { cookies } from "next/headers";
import { revalidatePath } from "next/cache";
import { THEME_COOKIE, type Theme } from "@/lib/theme";

/** setTheme enregistre le choix et redessine la page dans la foulée. */
export async function setTheme(theme: Theme) {
  const store = await cookies();

  if (theme === "system") {
    // Aucun cookie : c'est le réglage du système qui reprend la main.
    store.delete(THEME_COOKIE);
  } else {
    store.set(THEME_COOKIE, theme, {
      httpOnly: false,
      sameSite: "lax",
      path: "/",
      maxAge: 365 * 24 * 60 * 60,
    });
  }

  revalidatePath("/", "layout");
}
