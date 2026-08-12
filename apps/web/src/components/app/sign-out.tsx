import { signOut } from "@/app/actions";

/** Bouton de déconnexion. Un formulaire suffit : aucun état client n'est utile. */
export function SignOutButton() {
  return (
    <form action={signOut}>
      <button
        type="submit"
        className="mt-2 w-full rounded-md px-2 py-1 text-left text-2xs text-ink-3 transition-colors duration-[120ms] hover:bg-surface-2 hover:text-ink"
      >
        Se déconnecter
      </button>
    </form>
  );
}
