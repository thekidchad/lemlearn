import type { Metadata } from "next";
import Link from "next/link";
import { JournalShell } from "@/components/app/journal-shell";
import { apiFetch } from "@/lib/api";

export const metadata: Metadata = { title: "Courriels" };

interface Entry {
  sentAt: string;
  to: string;
  subject: string;
  template?: string;
  orgId?: string;
  /** Le relais a pris le message en charge — ce n'est pas « reçu ». */
  accepted: boolean;
  providerId?: string;
  error?: string;
  provider: string;
}

const TEMPLATES: Record<string, string> = {
  "signature.invitation": "Document à signer",
  "signature.otp": "Code de signature",
  "learner.invitation": "Accès apprenant",
  "survey.cold": "Satisfaction à froid",
};

/**
 * Journal des courriels partis.
 *
 * C'est la réponse à « il dit n'avoir rien reçu » : le message est-il parti,
 * quand, à quelle adresse, et l'expéditeur l'a-t-il accepté. Le corps n'y
 * figure pas — il porte des liens de signature et des codes à usage unique,
 * qu'un journal consultable n'a pas à conserver.
 */
export default async function MailJournalPage({
  searchParams,
}: PageProps<"/admin/journal/courriels">) {
  const params = await searchParams;
  const orgId = typeof params.orgId === "string" ? params.orgId : "";
  const query = typeof params.q === "string" ? params.q : "";

  const search = new URLSearchParams();
  if (orgId) search.set("orgId", orgId);
  if (query) search.set("q", query);

  const data = await apiFetch<{
    entries: Entry[];
    delivered: number;
    failed: number;
  }>(`/v1/admin/emails${search.size > 0 ? `?${search}` : ""}`);

  return (
    <JournalShell
      courant="/admin/journal/courriels"
      chapeau={`Ce qui est parti et ce qui a échoué — ${data.delivered} acceptés par le relais, ${data.failed} en échec. C'est la première réponse à « il dit n'avoir rien reçu ».`}
    >
      <div className="px-8 py-6">
        <form className="flex flex-wrap items-center gap-2">
          {orgId && <input type="hidden" name="orgId" value={orgId} />}
          <input
            name="q"
            defaultValue={query}
            placeholder="Adresse ou objet"
            className="h-9 w-64 rounded-lg border border-line bg-surface-0 px-3 text-sm outline-none focus:border-accent"
          />
          <button
            type="submit"
            className="btn-secondary"
          >
            Filtrer
          </button>
          {(orgId || query) && (
            <Link
              href="/admin/journal/courriels"
              className="text-2xs text-ink-3 underline hover:text-ink"
            >
              Tout voir
            </Link>
          )}
          <Link
            href="/admin/gabarits"
            className="ml-auto text-2xs text-ink-3 underline hover:text-ink"
          >
            Modifier les gabarits
          </Link>
        </form>

        {data.entries.length === 0 ? (
          <p className="mt-10 text-center text-xs text-ink-3">
            Aucun envoi sur les deux derniers mois.
          </p>
        ) : (
          <div className="mt-5 overflow-x-auto rounded-xl border border-line">
            <table className="w-full text-left text-xs">
              <thead className="border-b border-line text-2xs tracking-wide text-ink-3 uppercase">
                <tr>
                  <th className="px-4 py-2.5 font-medium">Envoyé</th>
                  <th className="px-4 py-2.5 font-medium">Destinataire</th>
                  <th className="px-4 py-2.5 font-medium">Objet</th>
                  <th className="px-4 py-2.5 font-medium">Type</th>
                  <th className="px-4 py-2.5 font-medium">État</th>
                  <th className="px-4 py-2.5 font-medium">Référence</th>
                </tr>
              </thead>
              <tbody>
                {data.entries.map((entry) => (
                  <tr
                    key={`${entry.sentAt}-${entry.to}`}
                    className="border-b border-line/60 last:border-0"
                  >
                    <td className="px-4 py-2 font-mono text-2xs whitespace-nowrap text-ink-3">
                      {new Date(entry.sentAt).toLocaleString("fr-FR")}
                    </td>
                    <td className="px-4 py-2 font-mono text-2xs">{entry.to}</td>
                    <td className="max-w-xs truncate px-4 py-2 text-ink-2">{entry.subject}</td>
                    <td className="px-4 py-2 text-2xs text-ink-3">
                      {entry.template ? (TEMPLATES[entry.template] ?? entry.template) : "—"}
                    </td>
                    <td className="px-4 py-2">
                      {entry.accepted ? (
                        <span
                          className="text-2xs text-ok"
                          title={
                            entry.provider === "resend"
                              ? "Le relais a pris le message en charge. La remise dans la boîte du destinataire ne nous est pas rapportée."
                              : undefined
                          }
                        >
                          {entry.provider === "resend" ? "accepté" : "journalisé"}
                        </span>
                      ) : (
                        <span className="text-2xs text-danger" title={entry.error}>
                          échec
                        </span>
                      )}
                    </td>
                    <td className="px-4 py-2 font-mono text-2xs text-ink-3">
                      {entry.providerId ? entry.providerId.slice(0, 8) : "—"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        <p className="mt-4 text-2xs text-ink-3">
          « Accepté » veut dire que le relais a pris le message en charge, pas
          qu&apos;il est arrivé : sans notification de sa part, la remise, le
          rejet et le classement en indésirable nous sont invisibles. La
          référence permet de retrouver le message chez le relais.{" "}
        </p>
        <p className="mt-2 text-2xs text-ink-3">
          {/* Dire ce que le journal ne contient pas vaut mieux que de laisser
              quelqu'un le chercher. */}
          Le corps des messages n&apos;est pas conservé : il porte des liens de
          signature et des codes à usage unique. La trace vit treize mois — la
          durée pendant laquelle un financeur peut contester un dossier — puis
          disparaît d&apos;elle-même. Hors production, « journalisé » signifie
          qu&apos;aucun courriel n&apos;est réellement parti.
        </p>
      </div>
    </JournalShell>
  );
}
