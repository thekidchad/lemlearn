import type { Metadata } from "next";
import { apiFetch } from "@/lib/api";

export const metadata: Metadata = { title: "Mes documents" };

interface Piece {
  id: string;
  kind: string;
  reference: string;
  status: string;
  signedAt?: string;
  sha256?: string;
  downloadable: boolean;
}

const KINDS: Record<string, string> = {
  convention: "Convention de formation",
  devis: "Devis",
  contrat: "Contrat de formation",
  attestation: "Attestation de fin de formation",
  emargement: "Feuille d'émargement",
};

const STATUTS: Record<string, string> = {
  pending: "À signer",
  opened: "Ouvert, pas encore signé",
  otp_sent: "Code envoyé, signature à confirmer",
  signed: "Signé",
  expired: "Lien expiré",
  revoked: "Annulé",
};

/**
 * Les pièces signées, du côté de celui qui les a signées.
 *
 * Une signature électronique ne vaut que si le signataire garde sa copie. La
 * lui faire réclamer par courriel revient à lui demander de faire confiance —
 * c'est précisément ce que la chaîne de preuve est censée remplacer.
 *
 * L'empreinte est affichée avec chaque pièce. Elle n'est pas décorative : c'est
 * le nombre qu'un contrôleur recalcule sur le fichier, et le seul moyen pour
 * l'apprenant de vérifier que sa copie est bien celle qui a été scellée.
 */
export default async function DocumentsPage() {
  const { pieces } = await apiFetch<{ pieces: Piece[] | null }>("/v1/learn/moi");
  const rows = pieces ?? [];

  return (
    <div className="mx-auto max-w-2xl px-5 py-12 sm:px-8 sm:py-16">
      <h1 className="learner-title">Mes documents</h1>
      <p className="learner-body mt-3">
        Les pièces que vous avez signées, et celles qui vous attendent.
      </p>

      {rows.length === 0 ? (
        <p className="mt-10 rounded-xl border border-line p-6 text-sm text-ink-2">
          Aucun document à votre nom pour l&apos;instant. La convention de
          formation apparaîtra ici dès que votre organisme vous l&apos;aura
          envoyée à signer.
        </p>
      ) : (
        <ul className="mt-8 space-y-3">
          {rows.map((piece) => (
            <li key={piece.id} className="rounded-xl border border-line p-5">
              <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                <h2 className="learner-heading">{KINDS[piece.kind] ?? piece.kind}</h2>
                {piece.reference && (
                  <span className="font-mono text-xs text-ink-3">{piece.reference}</span>
                )}
              </div>

              <p className="mt-1 text-sm text-ink-2">
                {STATUTS[piece.status] ?? piece.status}
                {piece.signedAt ? ` le ${new Date(piece.signedAt).toLocaleDateString("fr-FR")}` : ""}
              </p>

              {piece.sha256 && (
                <p className="mt-2 font-mono text-2xs break-all text-ink-3">
                  empreinte {piece.sha256}
                </p>
              )}

              {piece.downloadable ? (
                <a
                  href={`/api/apprenant/documents/${piece.id}`}
                  className="mt-4 inline-flex h-11 items-center rounded-xl border border-line px-5 text-sm hover:border-accent"
                >
                  Télécharger le PDF signé
                </a>
              ) : (
                <p className="mt-3 text-sm text-ink-3">
                  Le document sera téléchargeable ici une fois signé.
                </p>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
