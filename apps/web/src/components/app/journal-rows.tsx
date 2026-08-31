"use client";

import { useCallback, useEffect, useState } from "react";

/**
 * L'activité de la plateforme, dans l'ordre du temps.
 *
 * Ce n'est pas un tableau de bord : on n'y vient pas pour contempler, on y
 * vient avec une question — qui est entré sur ce compte, d'où, à quelle heure,
 * qui a exporté ce dossier, pourquoi cette signature n'aboutit pas. L'écran est
 * donc fait de filtres et d'une liste, et rien d'autre.
 *
 * L'adresse IP et le navigateur sont affichés à côté de l'auteur, pas cachés
 * derrière un détail à déplier : ce sont eux qui distinguent une connexion
 * ordinaire d'une qui ne l'est pas, et un renseignement qu'il faut cliquer
 * pour voir n'est pas lu.
 */
interface Ligne {
  at: string;
  action: string;
  subject: string;
  seq: number;
  actor: {
    type?: string;
    id?: string;
    label?: string;
    ip?: string;
    userAgent?: string;
  };
  payload?: Record<string, unknown>;
}

/** Les familles d'événements, dans l'ordre où on les cherche. */
const FAMILLES = [
  { clef: "", label: "Tout" },
  { clef: "auth", label: "Connexions" },
  { clef: "admin", label: "Accès de l'équipe" },
  { clef: "signature", label: "Signatures" },
  { clef: "document", label: "Documents" },
  { clef: "file", label: "Dossiers" },
  { clef: "quiz", label: "Questionnaires" },
  { clef: "watch", label: "Visionnage" },
  { clef: "attendance", label: "Émargements" },
  { clef: "dossier", label: "Exports" },
  { clef: "billing", label: "Facturation" },
] as const;

const ACTIONS: Record<string, string> = {
  "auth.signed_in": "Connexion",
  "auth.sign_in_failed": "Connexion refusée",
  "auth.signed_out": "Déconnexion",
  "admin.impersonated": "Accès de l'équipe lemlearn",
  "file.created": "Dossier ouvert",
  "file.stage_changed": "Dossier déplacé",
  "consent.given": "Consentement recueilli",
  "document.generated": "Document produit",
  "document.sent": "Document envoyé",
  "signature.opened": "Document ouvert par le signataire",
  "signature.otp_sent": "Code de signature envoyé",
  "signature.otp_verified": "Code vérifié",
  "signature.otp_failed": "Code refusé",
  "document.signed": "Document signé",
  "document.sealed": "Document scellé",
  "watch.progress": "Visionnage",
  "module.completed": "Module terminé",
  "quiz.started": "Questionnaire commencé",
  "quiz.submitted": "Questionnaire rendu",
  "attendance.signed": "Émargement signé",
  "session.closed": "Session clôturée",
  "followup.scheduled": "Relance programmée",
  "certificate.issued": "Attestation délivrée",
  "dossier.exported": "Dossier exporté",
  "learner.anonymized": "Apprenant anonymisé",
  "billing.plan_changed": "Formule changée",
};

/** Les actions dont on veut voir tout de suite qu'elles ont mal tourné. */
const ALERTES = new Set([
  "auth.sign_in_failed",
  "signature.otp_failed",
  "admin.impersonated",
]);

export function JournalRows() {
  const [rows, setRows] = useState<Ligne[]>([]);
  const [cursor, setCursor] = useState("");
  const [jusqua, setJusqua] = useState("");
  const [famille, setFamille] = useState("");
  const [terme, setTerme] = useState("");
  const [applique, setApplique] = useState({ famille: "", terme: "" });
  const [pret, setPret] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const adresse = useCallback(
    (curseur: string, filtres: { famille: string; terme: string }) => {
      const url = new URL("/api/admin/journal", window.location.origin);
      url.searchParams.set("limite", "60");
      if (filtres.famille) url.searchParams.set("famille", filtres.famille);
      if (filtres.terme) url.searchParams.set("q", filtres.terme);
      if (curseur) url.searchParams.set("curseur", curseur);
      return url;
    },
    [],
  );

  // Le chargement dépend des filtres appliqués, pas de ce qui est en train
  // d'être tapé : relancer à chaque touche ferait défiler la liste sous les
  // doigts et coûterait une lecture par caractère.
  useEffect(() => {
    const controller = new AbortController();
    (async () => {
      try {
        const response = await fetch(adresse("", applique), { signal: controller.signal });
        const body = (await response.json()) as {
          lignes?: Ligne[] | null;
          cursor?: string;
          jusqua?: string;
          error?: string;
        };
        if (!response.ok) throw new Error(body.error ?? "lecture impossible");
        setRows(body.lignes ?? []);
        setCursor(body.cursor ?? "");
        setJusqua(body.jusqua ?? "");
        setError(null);
      } catch (failure) {
        if (controller.signal.aborted) return;
        setError(failure instanceof Error ? failure.message : "lecture impossible");
      } finally {
        if (!controller.signal.aborted) setPret(true);
      }
    })();
    return () => controller.abort();
  }, [adresse, applique]);

  const suite = async () => {
    setBusy(true);
    try {
      const response = await fetch(adresse(cursor, applique));
      const body = (await response.json()) as {
        lignes?: Ligne[] | null;
        cursor?: string;
        jusqua?: string;
        error?: string;
      };
      if (!response.ok) throw new Error(body.error ?? "lecture impossible");
      setRows((precedents) => [...precedents, ...(body.lignes ?? [])]);
      setCursor(body.cursor ?? "");
      setJusqua(body.jusqua ?? "");
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "lecture impossible");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="px-8 py-6">
      <form
        className="flex flex-wrap items-center gap-2"
        onSubmit={(event) => {
          event.preventDefault();
          setApplique({ famille, terme });
        }}
      >
        <div className="flex flex-wrap gap-1">
          {FAMILLES.map((choix) => (
            <button
              key={choix.clef || "tout"}
              type="button"
              aria-current={choix.clef === applique.famille ? "page" : undefined}
              onClick={() => {
                setFamille(choix.clef);
                setApplique({ famille: choix.clef, terme });
              }}
              className={`rounded-md px-2.5 py-1 text-xs transition-colors duration-[120ms] ${
                choix.clef === applique.famille
                  ? "bg-surface-2 text-ink"
                  : "text-ink-3 hover:bg-surface-2 hover:text-ink"
              }`}
            >
              {choix.label}
            </button>
          ))}
        </div>

        <div className="ml-auto flex items-center gap-2">
          <input
            type="search"
            value={terme}
            onChange={(event) => setTerme(event.target.value)}
            placeholder="Adresse IP, compte, sujet…"
            className="h-8 w-64 rounded-md border border-line bg-surface-0 px-3 text-xs outline-none placeholder:text-ink-3 focus:border-accent"
          />
          <button type="submit" className="btn-secondary">
            Filtrer
          </button>
        </div>
      </form>

      {!pret ? (
        <p className="py-20 text-center text-xs text-ink-3">Lecture du journal…</p>
      ) : error ? (
        <p className="py-20 text-center text-xs text-danger">{error}</p>
      ) : rows.length === 0 ? (
        <p className="py-20 text-center text-xs text-ink-3">
          Rien de tel {jusqua ? `depuis le ${frDate(jusqua)}` : "sur la période regardée"}.
        </p>
      ) : (
        <div className="mt-5 overflow-hidden rounded-xl border border-line">
          <table className="w-full text-left text-xs">
            <thead className="border-b border-line text-2xs tracking-wide text-ink-3 uppercase">
              <tr>
                <th className="px-4 py-2.5 font-medium">Quand</th>
                <th className="px-4 py-2.5 font-medium">Événement</th>
                <th className="px-4 py-2.5 font-medium">Qui</th>
                <th className="px-4 py-2.5 font-medium">Depuis</th>
                <th className="px-4 py-2.5 font-medium">Sur quoi</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((ligne) => (
                <tr
                  key={`${ligne.subject}-${ligne.seq}-${ligne.at}`}
                  className="border-b border-line/60 last:border-0 hover:bg-surface-1"
                >
                  <td className="px-4 py-2 font-mono text-2xs whitespace-nowrap text-ink-3">
                    {new Date(ligne.at).toLocaleString("fr-FR")}
                  </td>
                  <td className="px-4 py-2">
                    <span className={ALERTES.has(ligne.action) ? "text-warn" : ""}>
                      {ACTIONS[ligne.action] ?? ligne.action}
                    </span>
                    {resume(ligne.payload) && (
                      <span className="ml-2 text-2xs text-ink-3">{resume(ligne.payload)}</span>
                    )}
                  </td>
                  <td className="max-w-[16rem] truncate px-4 py-2 text-ink-2">
                    {ligne.actor.label || ligne.actor.id || "—"}
                  </td>
                  <td className="px-4 py-2 font-mono text-2xs whitespace-nowrap text-ink-3">
                    {ligne.actor.ip || "—"}
                    {ligne.actor.userAgent && (
                      <span className="ml-2 text-ink-3/70" title={ligne.actor.userAgent}>
                        {navigateur(ligne.actor.userAgent)}
                      </span>
                    )}
                  </td>
                  <td className="max-w-[14rem] truncate px-4 py-2 font-mono text-2xs text-ink-3">
                    {ligne.subject}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>

          <div className="flex items-center gap-3 border-t border-line px-4 py-3">
            <p className="text-2xs text-ink-3" data-numeric>
              {rows.length} événement{rows.length > 1 ? "s" : ""}
              {jusqua ? ` — remonté jusqu'au ${frDate(jusqua)}` : ""}
            </p>
            {cursor && (
              <button
                type="button"
                className="btn-secondary ml-auto"
                disabled={busy}
                onClick={suite}
              >
                {busy ? "Chargement…" : "Remonter plus loin"}
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

/** Ce que le contenu de l'événement ajoute d'utile, en quelques mots. */
function resume(payload?: Record<string, unknown>): string {
  if (!payload) return "";
  for (const clef of ["motif", "compte", "recherche", "consultation", "fiche", "par", "role"]) {
    const valeur = payload[clef];
    if (typeof valeur === "string" && valeur !== "") return `— ${valeur}`;
  }
  return "";
}

/** Le navigateur, en un mot. Le reste tient dans l'infobulle. */
function navigateur(userAgent: string): string {
  if (/edg\//i.test(userAgent)) return "Edge";
  if (/chrome|crios/i.test(userAgent)) return "Chrome";
  if (/firefox/i.test(userAgent)) return "Firefox";
  if (/safari/i.test(userAgent)) return "Safari";
  if (/curl/i.test(userAgent)) return "curl";
  return "";
}

function frDate(jour: string): string {
  const [annee, mois, date] = jour.split("-");
  return `${date}/${mois}/${annee}`;
}
