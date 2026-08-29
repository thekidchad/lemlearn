import type { Metadata } from "next";
import Link from "next/link";
import { AttendanceSign, type SlotView } from "@/components/app/attendance-sign";
import { apiFetch } from "@/lib/api";

export const metadata: Metadata = { title: "Ma présence" };

interface Sheet {
  mode: string;
  slots: SlotView[] | null;
  trainerSignedAt?: string;
  trainerName?: string;
  opensBeforeMinute: number;
}

/**
 * Émargement de l'apprenant.
 *
 * C'est la pièce que réclame un financeur pour payer les heures : sans
 * signature du stagiaire, une feuille de présence n'atteste que ce que
 * l'organisme en dit. L'écran est volontairement pauvre — une liste de
 * créneaux et un bouton — parce qu'il est consulté en début de séance, souvent
 * sur un téléphone, par quelqu'un qui n'est pas venu pour ça.
 */
export default async function EmargementPage({
  params,
  searchParams,
}: PageProps<"/apprenant/[sessionId]/emargement">) {
  const { sessionId } = await params;
  const { contactId } = await searchParams;
  const suffix = typeof contactId === "string" ? `?contactId=${contactId}` : "";

  const sheet = await apiFetch<Sheet>(`/v1/learn/${sessionId}/emargement${suffix}`);
  const slots = sheet.slots ?? [];
  const signed = slots.filter((slot) => slot.signed).length;

  return (
    <div className="mx-auto max-w-2xl px-6 py-8">
      <Link href="/apprenant" className="text-xs text-ink-3 hover:text-ink">
        ← Mon parcours
      </Link>

      <p className="eyebrow mt-6">Ma présence</p>
      <h1 className="mt-1 text-xl font-semibold tracking-[-0.03em]">
        Émargement
      </h1>
      <p className="mt-2 text-sm text-ink-2">
        {sheet.mode === "async"
          ? "Votre présence est établie par votre relevé de connexion, puis contresignée par votre formateur. Vous n'avez rien à signer."
          : `Confirmez votre présence à chaque créneau. L'émargement ouvre ${sheet.opensBeforeMinute} minutes avant le début et se ferme peu après la fin : c'est ce qui lui donne sa valeur.`}
      </p>

      <p className="mt-4 text-2xs text-ink-3" data-numeric>
        {signed} créneau{signed > 1 ? "x" : ""} sur {slots.length} émargé
        {signed > 1 ? "s" : ""}
      </p>

      <div className="surface-card mt-4 p-5">
        <AttendanceSign
          sessionId={sessionId}
          contactId={typeof contactId === "string" ? contactId : undefined}
          slots={slots}
        />
      </div>

      {sheet.trainerSignedAt && (
        <p className="mt-4 text-2xs text-ink-3">
          Feuille contresignée
          {sheet.trainerName ? ` par ${sheet.trainerName}` : ""} le{" "}
          {new Date(sheet.trainerSignedAt).toLocaleDateString("fr-FR")}.
        </p>
      )}
    </div>
  );
}
