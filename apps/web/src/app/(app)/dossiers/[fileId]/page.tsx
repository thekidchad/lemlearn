import type { Metadata } from "next";
import Link from "next/link";
import { CreatePanel, Field, Select } from "@/components/app/form";
import { StagePicker } from "@/components/app/stage-picker";
import { issueSignature } from "@/app/actions/crm";
import { notFound } from "next/navigation";
import {
  apiFetch,
  ApiError,
  proofPercent,
  type AuditEvent,
  type Contact,
  type FileRecord,
  type SignatureRequest,
} from "@/lib/api";

export const metadata: Metadata = { title: "Dossier" };

/** Libellés des actions du journal, en français d'utilisateur. */
const ACTIONS: Record<string, string> = {
  "file.created": "Dossier créé",
  "file.stage_changed": "Étape modifiée",
  "consent.given": "Consentement recueilli",
  "document.generated": "Document généré",
  "document.sent": "Document envoyé à signer",
  "signature.opened": "Lien de signature ouvert",
  "signature.otp_sent": "Code de vérification envoyé",
  "signature.otp_verified": "Code vérifié",
  "signature.otp_failed": "Code incorrect",
  "document.signed": "Document signé",
  "document.sealed": "Document scellé",
  "watch.progress": "Visionnage enregistré",
  "module.completed": "Module validé",
  "quiz.started": "Questionnaire commencé",
  "quiz.submitted": "Questionnaire soumis",
  "attendance.signed": "Émargement",
  "certificate.issued": "Attestation délivrée",
  "dossier.exported": "Dossier exporté",
  "learner.anonymized": "Apprenant anonymisé",
};

/** Actions qui constituent une preuve, mises en évidence dans la timeline. */
const PROVING = new Set([
  "document.signed",
  "document.sealed",
  "module.completed",
  "attendance.signed",
  "certificate.issued",
]);

export default async function FilePage({ params }: PageProps<"/dossiers/[fileId]">) {
  const { fileId } = await params;

  let file: FileRecord;
  try {
    file = await apiFetch<FileRecord>(`/v1/files/${fileId}`);
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) notFound();
    throw error;
  }

  // Les trois appels sont indépendants : les enchaîner tripleraient le temps
  // d'affichage d'un écran que l'on ouvre toute la journée.
  const [timeline, signatures, learner] = await Promise.all([
    apiFetch<{ events: AuditEvent[] }>(`/v1/files/${fileId}/timeline`).catch(() => ({
      events: [] as AuditEvent[],
    })),
    apiFetch<{ requests: SignatureRequest[] }>(`/v1/files/${fileId}/signatures`).catch(() => ({
      requests: [] as SignatureRequest[],
    })),
    file.learnerId
      ? apiFetch<Contact>(`/v1/contacts/${file.learnerId}`).catch(() => null)
      : Promise.resolve(null),
  ]);

  const percent = proofPercent(file.proof);

  return (
    <>
      <header className="flex h-14 items-center gap-3 border-b border-line px-6">
        <Link href="/pipeline" className="text-xs text-ink-3 hover:text-ink">
          Pipeline
        </Link>
        <span className="text-ink-3">/</span>
        <span className="font-mono text-xs text-ink-2">{file.reference}</span>

        <div className="ml-auto flex items-center gap-2">
          <StagePicker fileId={file.id} stage={file.stage} />

          <CreatePanel
            label="Envoyer à signer"
            title="Envoyer un document à signer"
            action={issueSignature}
            submitLabel="Envoyer"
          >
            <input type="hidden" name="fileId" value={file.id} />
            <Select
              label="Document"
              name="kind"
              defaultValue="convention"
              options={[{ value: "convention", label: "Convention de formation" }]}
            />
            <Field
              label="Nom du signataire"
              name="signerName"
              required
              defaultValue={learner ? `${learner.firstName ?? ""} ${learner.lastName ?? ""}`.trim() : ""}
            />
            <Field
              label="Courriel du signataire"
              name="signerEmail"
              type="email"
              required
              defaultValue={learner?.email ?? ""}
              hint="Le code de confirmation part à cette adresse : elle vaut preuve d'identité."
            />
            <Select
              label="En qualité de"
              name="role"
              defaultValue="client"
              options={[
                { value: "client", label: "Apprenant ou client" },
                { value: "provider", label: "Organisme de formation" },
                { value: "funder", label: "Financeur" },
              ]}
            />
          </CreatePanel>
        </div>
      </header>

      <div className="px-6 py-6">
        <div className="flex flex-wrap items-start justify-between gap-6">
          <div>
            <h1 className="text-xl font-semibold tracking-[-0.03em]">
              {learner ? `${learner.firstName ?? ""} ${learner.lastName ?? ""}`.trim() : file.title}
            </h1>
            <p className="mt-1 text-xs text-ink-2">{file.title}</p>
          </div>

          <div className="w-52">
            <div className="flex items-baseline justify-between">
              <span className="text-2xs text-ink-3">Dossier de preuve</span>
              <span
                className={`text-sm font-semibold ${percent >= 80 ? "text-ok" : percent >= 40 ? "text-warn" : "text-ink-2"}`}
                data-numeric
              >
                {percent} %
              </span>
            </div>
            <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-surface-3">
              <div
                className={`h-full rounded-full ${percent >= 80 ? "bg-ok" : percent >= 40 ? "bg-warn" : "bg-ink-3"}`}
                style={{ width: `${percent}%` }}
              />
            </div>
            <p className="mt-1.5 text-2xs text-ink-3">
              {file.proof.present} pièce{file.proof.present > 1 ? "s" : ""} sur{" "}
              {file.proof.expected}
            </p>
          </div>
        </div>

        <div className="mt-8 grid gap-6 lg:grid-cols-[1fr_300px]">
          <section>
            <h2 className="mb-3 text-2xs font-medium tracking-wide text-ink-3 uppercase">
              Journal horodaté
            </h2>

            {timeline.events.length === 0 ? (
              <p className="text-xs text-ink-3">Aucun événement enregistré.</p>
            ) : (
              <ol className="relative space-y-3 border-l border-line pl-5">
                {timeline.events.map((event) => (
                  <li key={event.seq} className="relative">
                    <span
                      className={`absolute top-1.5 -left-[1.4375rem] size-1.5 rounded-full ring-4 ring-surface-0 ${
                        PROVING.has(event.action) ? "bg-ok" : "bg-ink-3"
                      }`}
                    />
                    <p className="text-xs text-ink">
                      {ACTIONS[event.action] ?? event.action}
                    </p>
                    <p className="font-mono text-2xs text-ink-3">
                      {formatDateTime(event.at)}
                      {event.actor.label ? ` · ${event.actor.label}` : ""}
                      {event.actor.ip ? ` · ${event.actor.ip}` : ""}
                    </p>
                    {event.actor.on_behalf_of && (
                      <p className="mt-0.5 text-2xs text-warn">
                        Action réalisée par l&apos;équipe lemlearn au nom de l&apos;organisme.
                      </p>
                    )}
                  </li>
                ))}
              </ol>
            )}

            <p className="mt-4 flex items-center gap-1.5 text-2xs text-ink-3">
              <span className="size-1 rounded-full bg-ok" />
              Chaîne vérifiée · {timeline.events.length} événement
              {timeline.events.length > 1 ? "s" : ""} · empreintes chaînées
            </p>
          </section>

          <aside className="space-y-5">
            {learner && (
              <section className="surface-card p-4">
                <h2 className="text-2xs font-medium tracking-wide text-ink-3 uppercase">
                  Identité
                </h2>
                <dl className="mt-3 space-y-2 text-xs">
                  <Row label="Nom" value={`${learner.firstName ?? ""} ${learner.lastName ?? ""}`} />
                  <Row label="Naissance" value={learner.birthDate} />
                  <Row label="E-mail" value={learner.email} />
                  <Row label="Téléphone" value={learner.phone} />
                  <Row
                    label="Adresse"
                    value={[learner.address?.line1, learner.address?.postalCode, learner.address?.city]
                      .filter(Boolean)
                      .join(" ")}
                  />
                </dl>
              </section>
            )}

            <section className="surface-card p-4">
              <h2 className="text-2xs font-medium tracking-wide text-ink-3 uppercase">
                Signatures
              </h2>
              {signatures.requests.length === 0 ? (
                <p className="mt-3 text-xs text-ink-3">Aucun document envoyé à signer.</p>
              ) : (
                <ul className="mt-3 space-y-2.5">
                  {signatures.requests.map((request) => (
                    <li key={request.id}>
                      <p className="flex items-center gap-1.5 text-xs">
                        <span
                          className={`size-1.5 shrink-0 rounded-full ${
                            request.status === "signed" ? "bg-ok" : "bg-warn"
                          }`}
                        />
                        {request.reference}
                      </p>
                      <p className="mt-0.5 pl-3 font-mono text-2xs text-ink-3">
                        {request.status === "signed" && request.proof
                          ? `scellé ${formatDateTime(request.proof.signedAt)}`
                          : `en attente · ${request.signerName}`}
                      </p>
                      {request.proof?.timestampTsa && (
                        <p className="pl-3 font-mono text-2xs text-ink-3">
                          horodaté par un tiers
                        </p>
                      )}
                    </li>
                  ))}
                </ul>
              )}
            </section>

            {file.proof.missing && file.proof.missing.length > 0 && (
              <section className="surface-card p-4">
                <h2 className="text-2xs font-medium tracking-wide text-ink-3 uppercase">
                  Pièces attendues
                </h2>
                <ul className="mt-3 space-y-1.5">
                  {file.proof.missing.map((piece) => (
                    <li key={piece} className="flex gap-2 text-2xs text-ink-2">
                      <span className="mt-1.5 size-1 shrink-0 rounded-full bg-ink-3/50" />
                      {piece}
                    </li>
                  ))}
                </ul>
              </section>
            )}
          </aside>
        </div>
      </div>
    </>
  );
}

function Row({ label, value }: { label: string; value?: string }) {
  if (!value?.trim()) return null;
  return (
    <div className="flex justify-between gap-3">
      <dt className="shrink-0 text-ink-3">{label}</dt>
      <dd className="truncate text-right text-ink">{value}</dd>
    </div>
  );
}

function formatDateTime(iso: string): string {
  return new Intl.DateTimeFormat("fr-FR", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    timeZone: "Europe/Paris",
  }).format(new Date(iso));
}
