"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

/**
 * Ouverture de l'espace d'un organisme client.
 *
 * Aucun mot de passe n'est choisi ici : le compte du responsable est créé
 * désactivé, et il choisira le sien par le lien reçu. Un secret que nous
 * aurions fabriqué et envoyé par courriel resterait dans sa boîte pour
 * toujours.
 *
 * L'espace existe dès l'ouverture, avant même sa première connexion : c'est ce
 * qui permet de l'habiller — logo, couleur, identité juridique — pour qu'il
 * trouve son enseigne en arrivant.
 */
export function OpenOrg() {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState<{ url: string; sentTo?: string; warning?: string } | null>(
    null,
  );

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setError(null);
    setBusy(true);
    try {
      const response = await fetch("/api/admin/orgs", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          orgName: String(form.get("orgName") ?? ""),
          email: String(form.get("email") ?? ""),
          firstName: String(form.get("firstName") ?? ""),
          lastName: String(form.get("lastName") ?? ""),
        }),
      });
      const body = (await response.json()) as {
        error?: string;
        invitationUrl?: string;
        sentTo?: string;
        warning?: string;
      };
      if (!response.ok || !body.invitationUrl) {
        throw new Error(body.error ?? "ouverture refusée");
      }
      setDone({ url: body.invitationUrl, sentTo: body.sentTo, warning: body.warning });
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "ouverture impossible");
    } finally {
      setBusy(false);
    }
  };

  if (!open) {
    return (
      <button type="button" className="btn-primary" onClick={() => setOpen(true)}>
        Ouvrir un organisme
      </button>
    );
  }

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center bg-black/40 px-4 pt-[10vh]">
      <div className="w-full max-w-md rounded-xl border border-line bg-surface-1 p-5 shadow-2xl">
        <h2 className="text-sm font-medium">Ouvrir un organisme</h2>

        {done ? (
          <>
            <p className="mt-3 text-xs text-ink-2">
              {done.sentTo
                ? `L'invitation est partie à ${done.sentTo}. Le responsable choisira son mot de passe et entrera dans son espace.`
                : "L'espace est créé."}
            </p>
            {done.warning && (
              <p className="mt-3 rounded-lg border border-warn/40 bg-warn/10 px-3 py-2 text-2xs text-warn">
                {done.warning}
              </p>
            )}
            {/* Le lien est toujours affiché : si le courriel n'arrive pas, on
                le transmet à la main plutôt que de recommencer l'ouverture —
                qui échouerait, l'adresse étant désormais réservée. */}
            <p className="mt-3 text-2xs text-ink-3">
              Lien d&apos;invitation, à transmettre au besoin :
            </p>
            <code className="mt-1 block overflow-x-auto rounded-lg border border-line bg-surface-2 p-2 font-mono text-2xs">
              {done.url}
            </code>
            <p className="mt-3 text-2xs text-ink-3">
              Vous pouvez déjà l&apos;habiller depuis sa fiche : logo, couleur et
              identité juridique seront en place à sa première visite.
            </p>
            <button
              type="button"
              className="btn-secondary mt-4"
              onClick={() => {
                setOpen(false);
                setDone(null);
              }}
            >
              Fermer
            </button>
          </>
        ) : (
          <form onSubmit={submit}>
            <p className="mt-1.5 text-xs text-ink-2">
              Le responsable recevra un lien pour choisir son mot de passe. Aucun
              mot de passe n&apos;est fabriqué ici.
            </p>

            <label className="mt-4 block">
              <span className="eyebrow">Nom de l&apos;organisme</span>
              <input name="orgName" required className="field mt-1.5" />
            </label>

            <div className="mt-4 grid grid-cols-2 gap-3">
              <label className="block">
                <span className="eyebrow">Prénom</span>
                <input name="firstName" className="field mt-1.5" />
              </label>
              <label className="block">
                <span className="eyebrow">Nom</span>
                <input name="lastName" className="field mt-1.5" />
              </label>
            </div>

            <label className="mt-4 block">
              <span className="eyebrow">Adresse du responsable</span>
              <input name="email" type="email" required className="field mt-1.5" />
            </label>

            {error && <p className="mt-3 text-xs text-danger">{error}</p>}

            <div className="mt-5 flex gap-2">
              <button type="submit" className="btn-primary" disabled={busy}>
                {busy ? "Ouverture…" : "Ouvrir et inviter"}
              </button>
              <button type="button" className="btn-ghost" onClick={() => setOpen(false)}>
                Annuler
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
}
