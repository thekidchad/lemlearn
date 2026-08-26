import { signOut } from "@/app/actions";

/** Bouton de déconnexion. Un formulaire suffit : aucun état client n'est utile. */
export function SignOutButton() {
  return (
    <form action={signOut}>
      <button
        type="submit"
        className="h-7 w-full rounded-md border border-line px-2 text-2xs text-ink-3 transition-colors duration-[120ms] hover:border-line-strong hover:text-ink"
      >
        Se déconnecter
      </button>
    </form>
  );
}
