import { cookies } from "next/headers";
import type { Stage } from "@/lib/stages";

/**
 * Client de l'API Go.
 *
 * Les composants serveur appellent l'API directement, en relayant le cookie de
 * session du navigateur. Le jeton ne transite donc jamais par le JavaScript
 * du client : une faille XSS ne donne pas accès aux dossiers.
 */
const API_URL = process.env.LEMLEARN_API_URL ?? "http://localhost:8787";

/**
 * Nom du cookie que cette application pose sur son propre domaine.
 *
 * Le préfixe __Host- exige HTTPS : il ne peut donc pas servir en développement
 * local, servi en clair.
 */
export const SESSION_COOKIE =
  process.env.NODE_ENV === "production" ? "__Host-lemlearn_session" : "lemlearn_session";

/**
 * Nom du cookie attendu par l'API.
 *
 * Ce n'est pas forcément le même : l'API le choisit selon *son* environnement,
 * pas selon le nôtre. Une application locale branchée sur une API déployée doit
 * donc parler le nom de l'API — sans quoi chaque appel authentifié repart en
 * 401 sans que rien n'explique pourquoi.
 */
export const API_COOKIE = process.env.LEMLEARN_API_COOKIE ?? SESSION_COOKIE;

export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
  }
}

/**
 * apiFetch relaie la requête à l'API avec le cookie de session.
 *
 * `cache: "no-store"` est délibéré : un dossier, un pipeline ou un journal
 * d'audit ne se met jamais en cache. Afficher un état périmé dans un outil de
 * preuve est pire que de recharger.
 */
export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const store = await cookies();
  const session = store.get(SESSION_COOKIE);

  const response = await fetch(`${API_URL}${path}`, {
    ...init,
    cache: "no-store",
    headers: {
      "Content-Type": "application/json",
      ...(session ? { Cookie: `${API_COOKIE}=${session.value}` } : {}),
      ...init?.headers,
    },
  });

  if (!response.ok) {
    let message = `l'API a répondu ${response.status}`;
    try {
      const body = (await response.json()) as { error?: string };
      if (body.error) message = body.error;
    } catch {
      // Réponse non JSON : le message par défaut fera l'affaire.
    }
    throw new ApiError(response.status, message);
  }

  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

/**
 * apiText relaie une requête dont la réponse n'est pas du JSON.
 *
 * Un manifeste HLS est du texte, et le lecteur le veut tel quel — le passer
 * par un encodage JSON ne ferait que le casser.
 */
export async function apiText(path: string): Promise<{ body: string; contentType: string }> {
  const store = await cookies();
  const session = store.get(SESSION_COOKIE);

  const response = await fetch(`${API_URL}${path}`, {
    cache: "no-store",
    headers: session ? { Cookie: `${API_COOKIE}=${session.value}` } : {},
  });

  const body = await response.text();
  if (!response.ok) {
    let message = `l'API a répondu ${response.status}`;
    try {
      const parsed = JSON.parse(body) as { error?: string };
      if (parsed.error) message = parsed.error;
    } catch {
      // Réponse non JSON : le message par défaut fera l'affaire.
    }
    throw new ApiError(response.status, message);
  }

  return {
    body,
    contentType: response.headers.get("Content-Type") ?? "text/plain; charset=utf-8",
  };
}

/**
 * apiRaw relaie une requête dont la réponse est binaire — un PDF, par exemple.
 *
 * La réponse de fetch est renvoyée telle quelle : la recopier dans un tampon
 * ferait transiter un document de plusieurs mégaoctets par la mémoire du
 * serveur pour rien.
 */
export async function apiRaw(path: string, init?: RequestInit): Promise<Response> {
  const store = await cookies();
  const session = store.get(SESSION_COOKIE);

  return fetch(`${API_URL}${path}`, {
    ...init,
    cache: "no-store",
    headers: {
      ...(session ? { Cookie: `${API_COOKIE}=${session.value}` } : {}),
      ...init?.headers,
    },
  });
}

/** Indique si une session est présente, sans appeler l'API. */
export async function hasSession(): Promise<boolean> {
  const store = await cookies();
  return store.has(SESSION_COOKIE);
}

// --- Types partagés avec l'API -------------------------------------------
// Ils reflètent les structures Go. À terme, ils seront générés depuis
// l'OpenAPI plutôt que réécrits.

export type Role = "owner" | "admin" | "trainer" | "learner" | "superadmin";

/**
 * L'identité visible d'un organisme de formation.
 *
 * Elle est toujours complète : l'API résout les valeurs par défaut avant de
 * répondre, pour qu'aucun écran n'ait à se demander quoi afficher quand un
 * champ manque. `logoUrl` seule peut être vide — c'est le cas d'un organisme
 * qui n'a pas encore déposé de fichier, et le monogramme prend le relais.
 */
export interface Brand {
  name: string;
  logoUrl?: string;
  monogram: string;
  accent: string;
  accentInk: string;
  supportEmail?: string;
  /** Thème par défaut de l'organisme : "system", "light" ou "dark". */
  theme: string;
}

export interface Me {
  user: {
    id: string;
    orgId: string;
    contactId?: string;
    email: string;
    firstName: string;
    lastName: string;
    role: Role;
  };
  org: {
    id: string;
    name: string;
    plan: string;
    qualiopiCertified: boolean;
    /** Exonération de l'article 261-4-4° a du CGI : la facture ne porte alors aucune taxe. */
    vatExempt?: boolean;
  };
  brand: Brand;
  impersonatedBy: string;
  /**
   * Ce qui manque à l'identité juridique de l'organisme.
   *
   * Il voyage avec la session parce que l'avertissement doit être là dès le
   * premier écran : le chercher séparément le ferait apparaître une seconde
   * trop tard, après que la page s'est affichée.
   */
  legal?: {
    complete: boolean;
    missing?: { champ: string; label: string; pourquoi: string }[] | null;
  };
}



export interface ProofStatus {
  expected: number;
  present: number;
  missing?: string[];
}

export interface FileRecord {
  id: string;
  reference: string;
  title: string;
  stage: Stage;
  learnerId?: string;
  priceHT: number;
  vatRate: number;
  tags?: string[];
  /** Origine des fonds, telle que la ventile le bilan annuel. */
  funding?: string;
  proof: ProofStatus;
  createdAt: string;
  updatedAt: string;
}

export interface Contact {
  id: string;
  kind: "learner" | "company" | "funder";
  firstName?: string;
  lastName?: string;
  companyName?: string;
  email?: string;
  phone?: string;
  birthDate?: string;
  birthPlace?: string;
  siret?: string;
  position?: string;
  notes?: string;
  anonymized?: boolean;
  /** D'où vient la personne, et le jour où elle est devenue cliente. */
  marketingSource?: string;
  convertedOn?: string;
  identityDocKey?: string;
  address?: { line1?: string; postalCode?: string; city?: string; country?: string };
}

/** Libellé d'un contact : raison sociale pour une entreprise, nom sinon. */
export function contactName(contact: Contact): string {
  if (contact.kind !== "learner" && contact.companyName) return contact.companyName;
  return [contact.firstName, contact.lastName].filter(Boolean).join(" ") ||
    contact.companyName ||
    contact.email ||
    contact.id;
}

export interface AuditEvent {
  seq: number;
  at: string;
  action: string;
  actor: { type: string; id: string; label?: string; ip?: string; on_behalf_of?: string };
  payload?: Record<string, unknown>;
  hash: string;
}

export interface SignatureRequest {
  id: string;
  reference: string;
  kind: string;
  role: string;
  signerName: string;
  signerEmail: string;
  status: "pending" | "opened" | "otp_sent" | "signed" | "cancelled";
  expiresAt: string;
  proof?: {
    signedAt: string;
    sealedSha256: string;
    timestampTsa?: string;
    sealed: boolean;
    ip: string;
  };
}

/** Libellés des étapes du pipeline, dans l'ordre du parcours commercial. */

export { STAGES, proofPercent } from "@/lib/stages";
export type { Stage } from "@/lib/stages";
