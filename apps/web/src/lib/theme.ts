/**
 * Choix du thème, conservé dans un cookie.
 *
 * Un cookie plutôt que localStorage : le serveur doit connaître le thème pour
 * rendre la page dans la bonne couleur du premier coup. Avec localStorage, la
 * page arrive en clair puis bascule — le clignotement blanc que tout le monde
 * reconnaît, et qui pique les yeux à six heures du matin.
 */
export const THEME_COOKIE = "lemlearn_theme";

export type Theme = "light" | "dark" | "system";
