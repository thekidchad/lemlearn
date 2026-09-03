"use client";

import { useCallback, useEffect, useState } from "react";

/**
 * Le suivi d'une fiche : ce qu'on a dit, ce qu'on doit faire, ce qu'on a reçu.
 *
 * Trois choses qu'une assistante de formation ouvre plus souvent que tout le
 * reste, et qui n'existaient nulle part. Sans elles, un échange téléphonique ne
 * laissait aucune trace, un rappel se notait sur un papier, et une attestation
 * employeur reçue par courriel restait dans une boîte.
 *
 * Les trois arrivent en un seul appel : elles s'affichent ensemble, et trois
 * requêtes pour un écran qui n'en montre qu'une à la fois feraient trois fois
 * le travail.
 */
interface Note {
  id: string;
  body: string;
  author: string;
  createdAt: string;
}

interface Rappel {
  id: string;
  title: string;
  dueOn: string;
  assigneeName?: string;
  comments?: string;
  doneAt?: string;
  doneBy?: string;
  author: string;
}

interface Piece {
  id: string;
  name: string;
  contentType?: string;
  sizeBytes?: number;
  author: string;
  createdAt: string;
}

type Onglet = "notes" | "rappels" | "pieces";

export function ContactSuivi({
  contactId,
  membres,
}: {
  contactId: string;
  /** L'équipe, pour assigner un rappel à quelqu'un. */
  membres: { id: string; nom: string }[];
}) {
  const [onglet, setOnglet] = useState<Onglet>("notes");
  const [notes, setNotes] = useState<Note[]>([]);
  const [rappels, setRappels] = useState<Rappel[]>([]);
  const [pieces, setPieces] = useState<Piece[]>([]);
  const [pret, setPret] = useState(false);
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const recharger = useCallback(async () => {
    const response = await fetch(`/api/contacts/${contactId}/suivi`);
    const body = (await response.json()) as {
      notes?: Note[] | null;
      rappels?: Rappel[] | null;
      pieces?: Piece[] | null;
      error?: string;
    };
    if (!response.ok) throw new Error(body.error ?? "lecture impossible");
    setNotes(body.notes ?? []);
    setRappels(body.rappels ?? []);
    setPieces(body.pieces ?? []);
  }, [contactId]);

  useEffect(() => {
    const controller = new AbortController();
    (async () => {
      try {
        await recharger();
      } catch (failure) {
        if (controller.signal.aborted) return;
        setError(failure instanceof Error ? failure.message : "lecture impossible");
      } finally {
        if (!controller.signal.aborted) setPret(true);
      }
    })();
    return () => controller.abort();
  }, [recharger]);

  const agir = async (clef: string, action: () => Promise<Response>) => {
    setError(null);
    setBusy(clef);
    try {
      const response = await action();
      if (!response.ok) {
        const body = (await response.json().catch(() => ({}))) as { error?: string };
        throw new Error(body.error ?? "action refusée");
      }
      await recharger();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "action refusée");
    } finally {
      setBusy(null);
    }
  };

  const ajouterNote = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const form = event.currentTarget;
    const body = String(new FormData(form).get("body") ?? "");
    void agir("note", async () => {
      const response = await fetch(`/api/contacts/${contactId}/notes`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ body }),
      });
      if (response.ok) form.reset();
      return response;
    });
  };

  const ajouterRappel = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    const assigneeId = String(data.get("assigneeId") ?? "");
    void agir("rappel", async () => {
      const response = await fetch(`/api/contacts/${contactId}/rappels`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          title: String(data.get("title") ?? ""),
          dueOn: String(data.get("dueOn") ?? ""),
          comments: String(data.get("comments") ?? ""),
          assigneeId,
          assigneeName: membres.find((membre) => membre.id === assigneeId)?.nom ?? "",
        }),
      });
      if (response.ok) form.reset();
      return response;
    });
  };

  // Le dépôt se fait en trois temps : on signe, le navigateur écrit
  // directement dans le compartiment, puis on enregistre la fiche. Le fichier
  // ne transite jamais par l'API — un PDF de dix mégaoctets n'a rien à faire
  // dans la mémoire d'une fonction.
  const deposer = async (file: File) => {
    setError(null);
    setBusy("piece");
    try {
      const reserve = await fetch(`/api/contacts/${contactId}/pieces`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ filename: file.name, contentType: file.type }),
      });
      const reserved = (await reserve.json()) as {
        uploadUrl?: string;
        key?: string;
        error?: string;
      };
      if (!reserve.ok || !reserved.uploadUrl || !reserved.key) {
        throw new Error(reserved.error ?? "dépôt refusé");
      }

      const put = await fetch(reserved.uploadUrl, {
        method: "PUT",
        headers: { "Content-Type": file.type },
        body: file,
      });
      if (!put.ok) throw new Error(`le dépôt a échoué (${put.status})`);

      const attach = await fetch(`/api/contacts/${contactId}/pieces`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          key: reserved.key,
          name: file.name,
          contentType: file.type,
          sizeBytes: file.size,
        }),
      });
      if (!attach.ok) {
        const body = (await attach.json().catch(() => ({}))) as { error?: string };
        throw new Error(body.error ?? "enregistrement refusé");
      }
      await recharger();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "dépôt impossible");
    } finally {
      setBusy(null);
    }
  };

  const ouvrir = async (piece: Piece) => {
    setError(null);
    setBusy(piece.id);
    try {
      const response = await fetch(`/api/contacts/${contactId}/pieces/${piece.id}`);
      const body = (await response.json()) as { url?: string; error?: string };
      if (!response.ok || !body.url) throw new Error(body.error ?? "lecture refusée");
      window.open(body.url, "_blank", "noopener");
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "lecture refusée");
    } finally {
      setBusy(null);
    }
  };

  const enRetard = rappels.filter(
    (rappel) => !rappel.doneAt && rappel.dueOn < new Date().toISOString().slice(0, 10),
  ).length;

  const ONGLETS: { clef: Onglet; label: string; compte: number }[] = [
    { clef: "notes", label: "Notes", compte: notes.length },
    { clef: "rappels", label: "Rappels", compte: rappels.filter((r) => !r.doneAt).length },
    { clef: "pieces", label: "Pièces jointes", compte: pieces.length },
  ];

  return (
    <section className="surface-card overflow-hidden">
      <header className="flex flex-wrap items-center gap-1 border-b border-line px-3 py-2">
        {ONGLETS.map((choix) => (
          <button
            key={choix.clef}
            type="button"
            aria-current={choix.clef === onglet ? "page" : undefined}
            onClick={() => setOnglet(choix.clef)}
            className={`rounded-md px-2.5 py-1 text-xs transition-colors duration-[120ms] ${
              choix.clef === onglet
                ? "bg-surface-2 text-ink"
                : "text-ink-3 hover:bg-surface-2 hover:text-ink"
            }`}
          >
            {choix.label}
            {choix.compte > 0 && (
              <span className="ml-1.5 font-mono text-2xs text-ink-3" data-numeric>
                {choix.compte}
              </span>
            )}
          </button>
        ))}
        {enRetard > 0 && (
          <span className="ml-auto text-2xs text-warn">
            {enRetard} rappel{enRetard > 1 ? "s" : ""} en retard
          </span>
        )}
      </header>

      {error && <p className="px-5 py-2 text-xs text-danger">{error}</p>}
      {!pret && <p className="px-5 py-8 text-center text-xs text-ink-3">Lecture…</p>}

      {pret && onglet === "notes" && (
        <>
          <form onSubmit={ajouterNote} className="border-b border-line px-5 py-4">
            <textarea
              name="body"
              rows={3}
              required
              placeholder="Ce qui s'est dit, ce qui a été convenu…"
              className="block w-full rounded-lg border border-line bg-surface-0 px-3 py-2 text-sm outline-none placeholder:text-ink-3 focus:border-accent"
            />
            <button type="submit" className="btn-primary mt-3" disabled={busy === "note"}>
              {busy === "note" ? "Enregistrement…" : "Ajouter la note"}
            </button>
          </form>

          {notes.length === 0 ? (
            <p className="px-5 py-8 text-center text-xs text-ink-3">
              Aucune note. Un échange qui ne laisse pas de trace est un échange
              que le collègue suivant devra refaire.
            </p>
          ) : (
            <ul className="divide-y divide-line/60">
              {notes.map((note) => (
                <li key={note.id} className="px-5 py-3">
                  <p className="text-sm whitespace-pre-wrap">{note.body}</p>
                  <p className="mt-1.5 flex items-center gap-2 font-mono text-2xs text-ink-3">
                    {note.author} · {new Date(note.createdAt).toLocaleString("fr-FR")}
                    <button
                      type="button"
                      className="btn-ghost"
                      disabled={busy !== null}
                      onClick={() =>
                        agir(note.id, () =>
                          fetch(`/api/contacts/${contactId}/notes/${note.id}`, {
                            method: "DELETE",
                          }),
                        )
                      }
                    >
                      Supprimer
                    </button>
                  </p>
                </li>
              ))}
            </ul>
          )}
        </>
      )}

      {pret && onglet === "rappels" && (
        <>
          <form onSubmit={ajouterRappel} className="border-b border-line px-5 py-4">
            <div className="grid gap-3 sm:grid-cols-2">
              <label className="block sm:col-span-2">
                <span className="eyebrow">À faire</span>
                <input
                  name="title"
                  required
                  placeholder="Relancer pour l'accord de prise en charge"
                  className="field mt-1.5"
                />
              </label>
              <label className="block">
                <span className="eyebrow">Échéance</span>
                <input name="dueOn" type="date" required className="field mt-1.5" />
              </label>
              <label className="block">
                <span className="eyebrow">Assigné à</span>
                <select name="assigneeId" defaultValue="" className="field mt-1.5">
                  <option value="">Personne en particulier</option>
                  {membres.map((membre) => (
                    <option key={membre.id} value={membre.id}>
                      {membre.nom}
                    </option>
                  ))}
                </select>
              </label>
              <label className="block sm:col-span-2">
                <span className="eyebrow">Précisions</span>
                <input name="comments" className="field mt-1.5" />
              </label>
            </div>
            <button type="submit" className="btn-primary mt-3" disabled={busy === "rappel"}>
              {busy === "rappel" ? "Enregistrement…" : "Poser le rappel"}
            </button>
          </form>

          {rappels.length === 0 ? (
            <p className="px-5 py-8 text-center text-xs text-ink-3">
              Aucun rappel sur cette fiche.
            </p>
          ) : (
            <ul className="divide-y divide-line/60">
              {rappels.map((rappel) => {
                const retard =
                  !rappel.doneAt && rappel.dueOn < new Date().toISOString().slice(0, 10);
                return (
                  <li key={rappel.id} className="flex flex-wrap items-center gap-x-3 gap-y-1 px-5 py-3">
                    <input
                      type="checkbox"
                      checked={Boolean(rappel.doneAt)}
                      disabled={busy !== null}
                      onChange={(event) =>
                        agir(rappel.id, () =>
                          fetch(`/api/contacts/${contactId}/rappels/${rappel.id}`, {
                            method: "PATCH",
                            headers: { "Content-Type": "application/json" },
                            body: JSON.stringify({ done: event.target.checked }),
                          }),
                        )
                      }
                      className="size-3.5 accent-[var(--color-accent)]"
                      aria-label={`Marquer « ${rappel.title} » comme fait`}
                    />
                    <div className="min-w-0 flex-1">
                      <p
                        className={`truncate text-sm ${rappel.doneAt ? "text-ink-3 line-through" : ""}`}
                      >
                        {rappel.title}
                      </p>
                      <p className="truncate font-mono text-2xs text-ink-3">
                        <span className={retard ? "text-warn" : ""}>
                          {new Date(rappel.dueOn).toLocaleDateString("fr-FR")}
                        </span>
                        {rappel.assigneeName ? ` · ${rappel.assigneeName}` : ""}
                        {rappel.comments ? ` · ${rappel.comments}` : ""}
                        {rappel.doneAt ? ` · fait par ${rappel.doneBy}` : ""}
                      </p>
                    </div>
                    <button
                      type="button"
                      className="btn-ghost"
                      disabled={busy !== null}
                      onClick={() =>
                        agir(rappel.id, () =>
                          fetch(`/api/contacts/${contactId}/rappels/${rappel.id}`, {
                            method: "DELETE",
                          }),
                        )
                      }
                    >
                      Supprimer
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
        </>
      )}

      {pret && onglet === "pieces" && (
        <>
          <div className="border-b border-line px-5 py-4">
            <label className="btn-secondary cursor-pointer">
              {busy === "piece" ? "Dépôt…" : "Déposer une pièce"}
              <input
                type="file"
                className="hidden"
                disabled={busy !== null}
                onChange={(event) => {
                  const file = event.target.files?.[0];
                  event.target.value = "";
                  if (file) void deposer(file);
                }}
              />
            </label>
            <p className="mt-2 text-2xs text-ink-3">
              Attestation employeur, accord de prise en charge, devis signé à la
              main. La pièce d&apos;identité, elle, se dépose plus bas : elle vit
              dans un compartiment chiffré à part et s&apos;efface après
              validation du dossier.
            </p>
          </div>

          {pieces.length === 0 ? (
            <p className="px-5 py-8 text-center text-xs text-ink-3">
              Aucune pièce jointe.
            </p>
          ) : (
            <ul className="divide-y divide-line/60">
              {pieces.map((piece) => (
                <li key={piece.id} className="flex flex-wrap items-center gap-x-3 gap-y-1 px-5 py-3">
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm">{piece.name}</p>
                    <p className="truncate font-mono text-2xs text-ink-3">
                      {poids(piece.sizeBytes)} · {piece.author} ·{" "}
                      {new Date(piece.createdAt).toLocaleDateString("fr-FR")}
                    </p>
                  </div>
                  <button
                    type="button"
                    className="btn-ghost"
                    disabled={busy !== null}
                    onClick={() => ouvrir(piece)}
                  >
                    Ouvrir
                  </button>
                  <button
                    type="button"
                    className="btn-ghost"
                    disabled={busy !== null}
                    onClick={() =>
                      agir(piece.id, () =>
                        fetch(`/api/contacts/${contactId}/pieces/${piece.id}`, {
                          method: "DELETE",
                        }),
                      )
                    }
                  >
                    Supprimer
                  </button>
                </li>
              ))}
            </ul>
          )}
        </>
      )}
    </section>
  );
}

/** Le poids d'un fichier, dans l'unité qui se lit. */
function poids(octets?: number): string {
  if (!octets) return "—";
  if (octets < 1024) return `${octets} o`;
  if (octets < 1024 * 1024) return `${Math.round(octets / 1024)} ko`;
  return `${(octets / 1024 / 1024).toFixed(1)} Mo`;
}
