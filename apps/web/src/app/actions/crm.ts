"use server";

import { revalidatePath } from "next/cache";
import { apiFetch, ApiError } from "@/lib/api";

/**
 * Actions de création du CRM et du catalogue.
 *
 * Elles vivent côté serveur : le cookie de session n'est jamais exposé au
 * JavaScript du client, et une faille XSS ne donne donc pas accès aux
 * dossiers. Chaque action renvoie un état plutôt que de lever, pour que le
 * formulaire affiche le motif du refus — souvent une règle métier utile
 * (« une session se termine avant d'avoir commencé »).
 */
export interface FormState {
  error?: string;
  ok?: boolean;
}

async function submit(
  path: string,
  body: unknown,
  revalidate: string,
): Promise<FormState> {
  try {
    await apiFetch(path, { method: "POST", body: JSON.stringify(body) });
    revalidatePath(revalidate);
    return { ok: true };
  } catch (error) {
    if (error instanceof ApiError) return { error: error.message };
    return { error: "erreur interne" };
  }
}

function text(form: FormData, field: string): string {
  return String(form.get(field) ?? "").trim();
}

function number(form: FormData, field: string): number {
  const value = Number(form.get(field));
  return Number.isFinite(value) ? value : 0;
}

// Une liste se saisit par virgules ou par lignes : imposer l'un des deux se
// paie en objectifs pédagogiques collés les uns aux autres.
function list(form: FormData, field: string): string[] {
  return text(form, field)
    .split(/[\n,;]/)
    .map((entry) => entry.trim())
    .filter(Boolean);
}

export async function createContact(_: FormState, form: FormData): Promise<FormState> {
  const kind = text(form, "kind");
  return submit(
    "/v1/contacts",
    {
      kind,
      firstName: text(form, "firstName"),
      lastName: text(form, "lastName"),
      birthDate: text(form, "birthDate"),
      birthPlace: text(form, "birthPlace"),
      companyName: text(form, "companyName"),
      siret: text(form, "siret"),
      email: text(form, "email"),
      phone: text(form, "phone"),
      position: text(form, "position"),
      address: {
        line1: text(form, "line1"),
        postalCode: text(form, "postalCode"),
        city: text(form, "city"),
        country: text(form, "country") || "France",
      },
    },
    `/contacts?kind=${kind}`,
  );
}

export async function createFile(_: FormState, form: FormData): Promise<FormState> {
  return submit(
    "/v1/files",
    {
      title: text(form, "title"),
      learnerId: text(form, "learnerId"),
      companyId: text(form, "companyId"),
      funderId: text(form, "funderId"),
      courseId: text(form, "courseId"),
      priceHT: number(form, "priceHT"),
      tags: list(form, "tags"),
    },
    "/pipeline",
  );
}

export async function createCourse(_: FormState, form: FormData): Promise<FormState> {
  return submit(
    "/v1/courses",
    {
      title: text(form, "title"),
      goal: text(form, "goal"),
      objectives: list(form, "objectives"),
      prerequisites: text(form, "prerequisites"),
      audience: text(form, "audience"),
      means: text(form, "means"),
      assessment: text(form, "assessment"),
      sanction: text(form, "sanction"),
      accessibility: text(form, "accessibility"),
      durationHours: number(form, "durationHours"),
      priceHT: number(form, "priceHT"),
      tags: list(form, "tags"),
      published: form.get("published") === "on",
    },
    "/catalogue",
  );
}

export async function addModule(_: FormState, form: FormData): Promise<FormState> {
  const courseId = text(form, "courseId");
  return submit(
    `/v1/courses/${courseId}/modules`,
    {
      title: text(form, "title"),
      summary: text(form, "summary"),
      position: number(form, "position"),
      assetId: text(form, "assetId"),
      // La durée est saisie en minutes : personne ne compte en millisecondes,
      // et l'API en a besoin pour calculer la couverture.
      durationMs: Math.round(number(form, "durationMinutes") * 60_000),
      quizId: text(form, "quizId"),
      minCoveragePercent: number(form, "minCoveragePercent"),
    },
    `/catalogue/${courseId}`,
  );
}

export async function createSession(_: FormState, form: FormData): Promise<FormState> {
  const starts = text(form, "startsAt");
  const ends = text(form, "endsAt");
  return submit(
    "/v1/sessions",
    {
      courseId: text(form, "courseId"),
      title: text(form, "title"),
      mode: text(form, "mode"),
      // L'heure saisie est locale : la convertir ici évite un décalage d'une
      // ou deux heures sur les convocations, qui se remarque tout de suite.
      startsAt: starts ? new Date(starts).toISOString() : null,
      endsAt: ends ? new Date(ends).toISOString() : null,
      location: text(form, "location"),
      capacity: number(form, "capacity"),
      tags: list(form, "tags"),
    },
    "/sessions",
  );
}

export async function enroll(_: FormState, form: FormData): Promise<FormState> {
  const sessionId = text(form, "sessionId");
  return submit(
    `/v1/sessions/${sessionId}/enrollments`,
    {
      contactId: text(form, "contactId"),
      fileId: text(form, "fileId"),
      // La nature du stagiaire et la période du contrat se saisissent ici,
      // pendant qu'on les connaît : les reconstituer au moment du bilan, un an
      // plus tard, revient à les inventer.
      traineeType: text(form, "traineeType"),
      contractStart: text(form, "contractStart"),
      contractEnd: text(form, "contractEnd"),
      hoursElearning: number(form, "hoursElearning"),
      hoursRemote: number(form, "hoursRemote"),
      hoursOnSite: number(form, "hoursOnSite"),
    },
    `/sessions/${sessionId}`,
  );
}

export async function closeSession(_: FormState, form: FormData): Promise<FormState> {
  const sessionId = text(form, "sessionId");
  return submit(`/v1/sessions/${sessionId}/close`, {}, `/sessions/${sessionId}`);
}

export async function issueSignature(_: FormState, form: FormData): Promise<FormState> {
  const fileId = text(form, "fileId");
  return submit(
    `/v1/files/${fileId}/signatures`,
    {
      kind: text(form, "kind") || "convention",
      signerName: text(form, "signerName"),
      signerEmail: text(form, "signerEmail"),
      role: text(form, "role") || "client",
    },
    `/dossiers/${fileId}`,
  );
}

export async function moveFile(fileId: string, stage: string): Promise<FormState> {
  try {
    await apiFetch(`/v1/files/${fileId}/stage`, {
      method: "PATCH",
      body: JSON.stringify({ stage }),
    });
    revalidatePath("/pipeline");
    revalidatePath(`/dossiers/${fileId}`);
    return { ok: true };
  } catch (error) {
    if (error instanceof ApiError) return { error: error.message };
    return { error: "erreur interne" };
  }
}

/**
 * Émargement d'un créneau par le formateur.
 *
 * L'apprenant signe depuis son espace ; cette voie sert au formateur qui tient
 * la feuille en salle, et elle enregistre qui a coché — un émargement sans
 * auteur ne prouve rien.
 */
export async function markAttendance(
  sessionId: string,
  slotId: string,
  contactId: string,
  method: "signature" | "connection" | "absent",
): Promise<FormState> {
  const result = await submit(
    `/v1/sessions/${sessionId}/attendance`,
    { slotId, contactId, method },
    `/sessions/${sessionId}`,
  );
  return result;
}

/** Contresignature de la feuille par le formateur. */
export async function countersign(_: FormState, form: FormData): Promise<FormState> {
  const sessionId = text(form, "sessionId");
  return submit(
    `/v1/sessions/${sessionId}/attendance/countersign`,
    {},
    `/sessions/${sessionId}`,
  );
}

/** Mise à jour d'une fiche contact. */
export async function updateContact(_: FormState, form: FormData): Promise<FormState> {
  const contactId = text(form, "contactId");
  try {
    await apiFetch(`/v1/contacts/${contactId}`, {
      method: "PATCH",
      body: JSON.stringify({
        firstName: text(form, "firstName"),
        lastName: text(form, "lastName"),
        birthDate: text(form, "birthDate"),
        birthPlace: text(form, "birthPlace"),
        companyName: text(form, "companyName"),
        siret: text(form, "siret"),
        email: text(form, "email"),
        phone: text(form, "phone"),
        position: text(form, "position"),
        notes: text(form, "notes"),
        address: {
          line1: text(form, "line1"),
          postalCode: text(form, "postalCode"),
          city: text(form, "city"),
          country: text(form, "country") || "France",
        },
      }),
    });
    revalidatePath(`/contacts/${contactId}`);
    return { ok: true };
  } catch (error) {
    if (error instanceof ApiError) return { error: error.message };
    return { error: "erreur interne" };
  }
}

/**
 * Anonymisation d'un apprenant au titre du RGPD.
 *
 * Les pièces probatoires survivent sous pseudonyme : les effacer priverait
 * l'organisme de la preuve d'une formation qu'il a réellement dispensée, et
 * qu'il doit pouvoir justifier pendant dix ans.
 */
export async function anonymizeContact(_: FormState, form: FormData): Promise<FormState> {
  const contactId = text(form, "contactId");
  const reason = text(form, "reason");
  if (!reason) {
    return { error: "Le motif est obligatoire : il figure au journal d'audit." };
  }
  return submit(
    `/v1/contacts/${contactId}/anonymize`,
    { reason },
    `/contacts/${contactId}`,
  );
}
