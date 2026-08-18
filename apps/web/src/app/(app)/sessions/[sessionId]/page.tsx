import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { AttendanceCell } from "@/components/app/attendance-cell";
import { CreatePanel } from "@/components/app/form";
import { CloseSessionButton } from "@/components/app/close-session";
import { CountersignButton } from "@/components/app/countersign";
import { enroll } from "@/app/actions/crm";
import { apiFetch, ApiError, contactName, type Contact } from "@/lib/api";

export const metadata: Metadata = { title: "Émargement" };

interface Slot {
  id: string;
  label: string;
  start: string;
  end: string;
  moduleId?: string;
}

interface Sheet {
  sessionId: string;
  mode: string;
  slots: Slot[];
  trainerName?: string;
  trainerSignedAt?: string;
}

interface Entry {
  slotId: string;
  contactId: string;
  method: "signature" | "connection" | "absent";
  signedAt: string;
  coveragePercent?: number;
  comment?: string;
}

interface Enrollment {
  contactId: string;
  status: string;
}

interface SheetResponse {
  sheet: Sheet;
  entries: Entry[] | null;
  enrollments: Enrollment[] | null;
  attendedHours: Record<string, number>;
}

/**
 * Feuille d'émargement : créneaux en colonnes, apprenants en lignes.
 *
 * C'est la grille que remplit un formateur, et celle que recompte un
 * contrôleur. Les trois états — signé, présence par connexion, absent — sont
 * distingués visuellement : une case vide n'est pas une absence, c'est un
 * créneau non traité, et la confusion se paie en audit.
 */
export default async function AttendancePage({ params }: PageProps<"/sessions/[sessionId]">) {
  const { sessionId } = await params;

  let data: SheetResponse;
  try {
    data = await apiFetch<SheetResponse>(`/v1/sessions/${sessionId}/attendance`);
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) notFound();
    throw error;
  }

  // Les inscriptions ne portent qu'un identifiant de contact. Afficher cet
  // identifiant serait illisible pour un formateur qui coche des présences :
  // on résout les noms en une requête, plutôt qu'une par apprenant. La même
  // liste alimente le formulaire d'inscription : saisir un identifiant à la
  // main est le genre de détail qui fait abandonner un outil.
  const { contacts } = await apiFetch<{ contacts: Contact[] | null }>(
    "/v1/contacts?kind=learner",
  ).catch(() => ({ contacts: null }));
  const names = new Map(
    (contacts ?? []).map((contact) => [
      contact.id,
      `${contact.firstName ?? ""} ${contact.lastName ?? ""}`.trim() || contact.id.slice(-8),
    ]),
  );

  const entries = data.entries ?? [];
  const enrollments = data.enrollments ?? [];
  const byCell = new Map(entries.map((entry) => [`${entry.slotId}|${entry.contactId}`, entry]));

  return (
    <>
      <header className="flex h-14 items-center gap-3 border-b border-line px-6">
        <Link href="/sessions" className="text-xs text-ink-3 hover:text-ink">
          Sessions
        </Link>
        <span className="text-ink-3">/</span>
        <span className="text-xs text-ink-2">Émargement</span>

        <div className="ml-auto flex items-center gap-2">
          <CreatePanel label="Inscrire" title="Inscrire un apprenant" action={enroll}>
            <input type="hidden" name="sessionId" value={sessionId} />
            <label className="block">
              <span className="mb-1 block text-2xs text-ink-3">Apprenant</span>
              <select
                name="contactId"
                required
                className="h-9 w-full rounded-lg border border-line bg-surface-0 px-2 text-sm outline-none focus:border-accent"
              >
                {(contacts ?? []).map((contact) => (
                  <option key={contact.id} value={contact.id}>
                    {contactName(contact)}
                  </option>
                ))}
              </select>
            </label>
            <label className="block">
              <span className="mb-1 block text-2xs text-ink-3">Dossier (facultatif)</span>
              <input
                name="fileId"
                placeholder="Identifiant du dossier"
                className="h-9 w-full rounded-lg border border-line bg-surface-0 px-3 font-mono text-xs outline-none focus:border-accent"
              />
              <span className="mt-1 block text-2xs text-ink-3">
                Sans dossier, l&apos;inscription existe mais n&apos;alimente aucune
                chaîne de preuve.
              </span>
            </label>
          </CreatePanel>

          <CloseSessionButton sessionId={sessionId} />
        </div>

        <span className="ml-3 text-2xs text-ink-3">
          {data.sheet.trainerSignedAt ? (
            <span className="flex items-center gap-1.5 text-ok">
              <span className="size-1.5 rounded-full bg-ok" />
              Contresignée par {data.sheet.trainerName}
            </span>
          ) : (
            <CountersignButton sessionId={sessionId} />
          )}
        </span>
      </header>

      {enrollments.length === 0 ? (
        <p className="px-6 py-16 text-center text-xs text-ink-3">
          Aucun apprenant inscrit à cette session.
        </p>
      ) : (
        <div className="overflow-x-auto px-6 py-6">
          <table className="min-w-full border-separate border-spacing-0 text-left">
            <thead>
              <tr>
                <th className="sticky left-0 z-10 border-b border-line bg-surface-0 py-2 pr-4 text-2xs font-medium tracking-wide text-ink-3 uppercase">
                  Apprenant
                </th>
                {data.sheet.slots.map((slot) => (
                  <th
                    key={slot.id}
                    className="border-b border-line px-3 py-2 text-2xs font-medium text-ink-3"
                  >
                    {slot.label}
                  </th>
                ))}
                <th className="border-b border-line px-3 py-2 text-right text-2xs font-medium tracking-wide text-ink-3 uppercase">
                  Heures
                </th>
              </tr>
            </thead>
            <tbody>
              {enrollments.map((enrollment) => (
                <tr key={enrollment.contactId}>
                  <td className="sticky left-0 z-10 border-b border-line/60 bg-surface-0 py-2.5 pr-4 text-xs text-ink">
                    {names.get(enrollment.contactId) ?? enrollment.contactId.slice(-8)}
                  </td>

                  {data.sheet.slots.map((slot) => {
                    const entry = byCell.get(`${slot.id}|${enrollment.contactId}`);
                    return (
                      <td key={slot.id} className="border-b border-line/60 px-3 py-2.5">
                        <AttendanceCell
                          sessionId={sessionId}
                          slotId={slot.id}
                          contactId={enrollment.contactId}
                          method={entry?.method}
                          coveragePercent={entry?.coveragePercent}
                        />
                      </td>
                    );
                  })}

                  <td
                    className="border-b border-line/60 px-3 py-2.5 text-right font-mono text-xs text-ink"
                    data-numeric
                  >
                    {(data.attendedHours[enrollment.contactId] ?? 0).toFixed(1)} h
                  </td>
                </tr>
              ))}
            </tbody>
          </table>

          <p className="mt-4 flex flex-wrap items-center gap-4 text-2xs text-ink-3">
            <Legend color="bg-ok" label="Signé par l'apprenant" />
            <Legend color="bg-accent" label="Présence établie par relevé de connexion" />
            <Legend color="bg-bad" label="Absence motivée" />
            <Legend color="bg-surface-3" label="Créneau non traité" />
          </p>
        </div>
      )}
    </>
  );
}


function Legend({ color, label }: { color: string; label: string }) {
  return (
    <span className="flex items-center gap-1.5">
      <span className={`size-2 rounded-sm ${color}`} />
      {label}
    </span>
  );
}
