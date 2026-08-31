"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";

/**
 * La console de l'équipe : une nature, sur toute la plateforme.
 *
 * Ce n'est pas le répertoire d'un organisme et cela ne doit pas y ressembler.
 * Un tableau à colonnes dirait « voici vos stagiaires » — or ce ne sont pas les
 * nôtres, ce sont ceux de nos clients, et c'est le client qui est l'unité utile
 * quand quelqu'un appelle. Les lignes sont donc regroupées par organisme, sous
 * une barre qui porte son nom, sa fiche et l'accès à son espace.
 *
 * Trois gestes, et ce sont ceux qu'on fait au téléphone : entrer chez
 * l'organisme, entrer chez la personne, sortir son dossier.
 */
interface Ligne {
  orgId: string;
  orgName: string;
  id: string;
  label: string;
  detail?: string;
  canImpersonate?: boolean;
  fileId?: string;
  fileReference?: string;
}

export function ConsoleRows({
  vue,
  aide,
}: {
  vue: "stagiaires" | "entreprises" | "financeurs";
  /** Ce qu'est cette nature, dit quand la plateforme n'en contient aucune. */
  aide: string;
}) {
  const [rows, setRows] = useState<Ligne[]>([]);
  const [cursor, setCursor] = useState("");
  const [orgs, setOrgs] = useState(0);
  const [filtre, setFiltre] = useState("");
  const [pret, setPret] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const adresse = useCallback(
    (curseur: string) => {
      const url = new URL("/api/admin/tout", window.location.origin);
      url.searchParams.set("vue", vue);
      url.searchParams.set("limite", "100");
      if (curseur) url.searchParams.set("curseur", curseur);
      return url;
    },
    [vue],
  );

  useEffect(() => {
    const controller = new AbortController();
    (async () => {
      try {
        const response = await fetch(adresse(""), { signal: controller.signal });
        const body = (await response.json()) as {
          lignes?: Ligne[] | null;
          cursor?: string;
          organismes?: number;
          error?: string;
        };
        if (!response.ok) throw new Error(body.error ?? "lecture impossible");
        setRows(body.lignes ?? []);
        setCursor(body.cursor ?? "");
        setOrgs(body.organismes ?? 0);
      } catch (failure) {
        if (controller.signal.aborted) return;
        setError(failure instanceof Error ? failure.message : "lecture impossible");
      } finally {
        if (!controller.signal.aborted) setPret(true);
      }
    })();
    return () => controller.abort();
  }, [adresse]);

  const suite = async () => {
    setError(null);
    setBusy(true);
    try {
      const response = await fetch(adresse(cursor));
      const body = (await response.json()) as {
        lignes?: Ligne[] | null;
        cursor?: string;
        error?: string;
      };
      if (!response.ok) throw new Error(body.error ?? "lecture impossible");
      setRows((precedents) => [...precedents, ...(body.lignes ?? [])]);
      setCursor(body.cursor ?? "");
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "lecture impossible");
    } finally {
      setBusy(false);
    }
  };

  // Le filtre ne porte que sur ce qui est chargé, et le dit : promettre une
  // recherche sur toute la base alors qu'on trie une page ferait conclure à
  // tort qu'une fiche n'existe pas.
  const groupes = useMemo(() => {
    const terme = filtre.trim().toLowerCase();
    const retenues = terme
      ? rows.filter((row) =>
          `${row.label} ${row.detail ?? ""} ${row.orgName}`.toLowerCase().includes(terme),
        )
      : rows;

    const paquets = new Map<string, { nom: string; lignes: Ligne[] }>();
    for (const row of retenues) {
      const paquet = paquets.get(row.orgId) ?? { nom: row.orgName, lignes: [] };
      paquet.lignes.push(row);
      paquets.set(row.orgId, paquet);
    }
    return [...paquets.entries()];
  }, [rows, filtre]);

  if (!pret) {
    return (
      <p className="px-8 py-20 text-center text-xs text-ink-3">
        Lecture de la plateforme…
      </p>
    );
  }

  if (error) {
    return <p className="px-8 py-20 text-center text-xs text-danger">{error}</p>;
  }

  if (rows.length === 0) {
    return (
      <div className="mx-auto max-w-md px-8 py-20 text-center">
        <p className="text-sm text-ink-2">
          Rien de ce type sur la plateforme, {orgs} organisme
          {orgs > 1 ? "s" : ""} parcouru{orgs > 1 ? "s" : ""}.
        </p>
        <p className="mt-3 text-xs text-ink-3">{aide}</p>
      </div>
    );
  }

  const affichees = groupes.reduce((somme, [, paquet]) => somme + paquet.lignes.length, 0);

  return (
    <div className="px-8 py-6">
      <div className="mb-5 flex flex-wrap items-center gap-3">
        <input
          type="search"
          value={filtre}
          onChange={(event) => setFiltre(event.target.value)}
          placeholder="Filtrer les lignes chargées"
          className="h-9 w-72 rounded-lg border border-line bg-surface-1 px-3 text-sm outline-none placeholder:text-ink-3 focus:border-accent"
        />
        <p className="text-2xs text-ink-3" data-numeric>
          {affichees} sur {rows.length} chargée{rows.length > 1 ? "s" : ""} ·{" "}
          {groupes.length} organisme{groupes.length > 1 ? "s" : ""}
          {cursor ? "" : " · plateforme entière"}
        </p>
      </div>

      <div className="space-y-6">
        {groupes.map(([orgId, paquet]) => (
          <Groupe key={orgId} orgId={orgId} nom={paquet.nom} lignes={paquet.lignes} />
        ))}
      </div>

      {groupes.length === 0 && (
        <p className="py-16 text-center text-xs text-ink-3">
          Aucune ligne chargée ne correspond à « {filtre} ».
        </p>
      )}

      {cursor && (
        <div className="mt-6 flex justify-center">
          <button type="button" className="btn-secondary" disabled={busy} onClick={suite}>
            {busy ? "Chargement…" : "Charger la suite"}
          </button>
        </div>
      )}
    </div>
  );
}

/** Un organisme et ce qu'il contient de cette nature. */
function Groupe({
  orgId,
  nom,
  lignes,
}: {
  orgId: string;
  nom: string;
  lignes: Ligne[];
}) {
  return (
    <section className="overflow-hidden rounded-xl border border-line bg-surface-1">
      <header className="flex flex-wrap items-center gap-3 border-b border-line px-4 py-3">
        <span
          aria-hidden
          className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-accent-dim text-xs font-medium text-accent-ink"
        >
          {monogramme(nom)}
        </span>
        <div className="min-w-0">
          <Link
            href={`/admin/${orgId}`}
            className="truncate text-sm font-medium hover:text-accent-ink hover:underline"
          >
            {nom}
          </Link>
          <p className="text-2xs text-ink-3" data-numeric>
            {lignes.length} ligne{lignes.length > 1 ? "s" : ""}
          </p>
        </div>
        <div className="ml-auto">
          <EntrerOrganisme orgId={orgId} />
        </div>
      </header>

      <ul className="divide-y divide-line/60">
        {lignes.map((ligne) => (
          <LigneRendue key={ligne.id} ligne={ligne} />
        ))}
      </ul>
    </section>
  );
}

/** Une personne, une entreprise, un financeur — et ce qu'on peut en faire. */
function LigneRendue({ ligne }: { ligne: Ligne }) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const entrer = async () => {
    setError(null);
    setBusy(true);
    try {
      const response = await fetch(
        `/api/admin/orgs/${ligne.orgId}/contacts/${ligne.id}/impersonate`,
        { method: "POST" },
      );
      const body = (await response.json()) as { landing?: string; error?: string };
      if (!response.ok) throw new Error(body.error ?? "ouverture impossible");
      router.push(body.landing ?? "/pipeline");
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "ouverture impossible");
      setBusy(false);
    }
  };

  return (
    <li className="group flex flex-wrap items-center gap-x-4 gap-y-1 px-4 py-3 transition-colors duration-[120ms] hover:bg-surface-2">
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm">{ligne.label}</p>
        {ligne.detail && (
          <p className="truncate font-mono text-2xs text-ink-3">{ligne.detail}</p>
        )}
      </div>

      {error && <span className="text-2xs text-danger">{error}</span>}

      <div className="flex shrink-0 items-center gap-2">
        {ligne.fileId ? (
          // Un lien, pas un appel : le navigateur enregistre l'archive
          // lui-même, sans faire transiter plusieurs mégaoctets par la
          // mémoire de la page.
          <a
            href={`/api/admin/orgs/${ligne.orgId}/dossiers/${ligne.fileId}/export`}
            className="h-7 rounded-md border border-transparent px-2 text-2xs leading-[1.6rem] text-ink-3 hover:border-line hover:text-ink group-hover:border-line"
            title={`Dossier ${ligne.fileReference}`}
          >
            Exporter le dossier
          </a>
        ) : (
          <span className="text-2xs text-ink-3/60">sans dossier</span>
        )}

        {ligne.canImpersonate ? (
          <button
            type="button"
            className="h-7 rounded-md border border-transparent px-2 text-2xs text-ink-3 hover:border-line hover:text-ink disabled:opacity-50 group-hover:border-line"
            disabled={busy}
            onClick={entrer}
            title="Ouvre une session sur ce compte. L'accès est tracé et visible du client."
          >
            {busy ? "Ouverture…" : "Ouvrir sa session"}
          </button>
        ) : (
          <span
            className="text-2xs text-ink-3/60"
            title="Aucun compte : la personne n'a pas été invitée, ou n'a pas encore choisi son mot de passe."
          >
            sans compte
          </span>
        )}
      </div>
    </li>
  );
}

/** Entrer dans l'espace de l'organisme lui-même. */
function EntrerOrganisme({ orgId }: { orgId: string }) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const entrer = async () => {
    setError(null);
    setBusy(true);
    try {
      const response = await fetch(`/api/admin/${orgId}/impersonate`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: "{}",
      });
      const body = (await response.json()) as { error?: string };
      if (!response.ok) throw new Error(body.error ?? "ouverture impossible");
      router.push("/pipeline");
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "ouverture impossible");
      setBusy(false);
    }
  };

  return (
    <div className="flex items-center gap-2">
      {error && <span className="text-2xs text-danger">{error}</span>}
      <button
        type="button"
        className="h-7 rounded-md border border-line px-2.5 text-2xs text-ink-2 hover:border-accent hover:text-ink disabled:opacity-50"
        disabled={busy}
        onClick={entrer}
        title="Ouvre une session sur cet organisme. L'accès est tracé et visible du client."
      >
        {busy ? "Ouverture…" : "Entrer chez l'organisme"}
      </button>
    </div>
  );
}

/** Deux lettres pour un organisme, sans logo à charger. */
function monogramme(nom: string): string {
  const mots = nom.split(/[\s—-]+/).filter(Boolean);
  if (mots.length === 0) return "—";
  if (mots.length === 1) return mots[0].slice(0, 2).toUpperCase();
  return (mots[0][0] + mots[1][0]).toUpperCase();
}
