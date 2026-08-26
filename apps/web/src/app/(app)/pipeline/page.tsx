import type { Metadata } from "next";
import Link from "next/link";
import { CreatePanel, Field } from "@/components/app/form";
import { createFile } from "@/app/actions/crm";
import { ProofMarks } from "@/components/app/proof-marks";
import {
  apiFetch,
  contactName,
  STAGES,
  type Contact,
  type FileRecord,
  type Stage,
} from "@/lib/api";

export const metadata: Metadata = { title: "Pipeline" };

interface PipelineResponse {
  pipeline: Record<Stage, FileRecord[] | null>;
}

export default async function PipelinePage() {
  const { pipeline } = await apiFetch<PipelineResponse>("/v1/files");
  // Les apprenants alimentent la liste du formulaire : saisir un identifiant
  // à la main est le genre de détail qui fait abandonner un CRM.
  const { contacts } = await apiFetch<{ contacts: Contact[] | null }>(
    "/v1/contacts?kind=learner",
  ).catch(() => ({ contacts: null }));
  const total = STAGES.reduce((sum, stage) => sum + (pipeline[stage.key]?.length ?? 0), 0);

  return (
    <>
      <header className="flex h-14 items-center justify-between border-b border-line px-6">
        <div className="flex items-center gap-2.5">
          <h1 className="text-sm font-medium">Pipeline</h1>
          <span
            className="rounded-md border border-line bg-surface-2 px-1.5 py-0.5 font-mono text-2xs text-ink-3"
            data-numeric
          >
            {total} dossier{total > 1 ? "s" : ""}
          </span>
        </div>

        <CreatePanel label="Nouveau dossier" title="Nouveau dossier" action={createFile}>
          <Field label="Intitulé" name="title" required placeholder="SSIAP 1 — Léa Bertrand" />
          <label className="block">
            <span className="mb-1 block text-2xs text-ink-3">Apprenant</span>
            <select
              name="learnerId"
              className="h-9 w-full rounded-lg border border-line bg-surface-0 px-2 text-sm outline-none focus:border-accent"
            >
              <option value="">—</option>
              {(contacts ?? []).map((contact) => (
                <option key={contact.id} value={contact.id}>
                  {contactName(contact)}
                </option>
              ))}
            </select>
          </label>
          <Field label="Prix HT (€)" name="priceHT" type="number" defaultValue={0} />
          <Field
            label="Étiquettes"
            name="tags"
            placeholder="présentiel, OPCO-EP"
            hint="Séparées par des virgules."
          />
        </CreatePanel>
      </header>

      {total === 0 ? (
        <EmptyState />
      ) : (
        <div className="grid gap-px overflow-x-auto bg-line md:grid-cols-2 xl:grid-cols-5">
          {STAGES.map((stage) => {
            const files = pipeline[stage.key] ?? [];
            return (
              <section key={stage.key} className="min-h-[calc(100vh-3.5rem)] bg-surface-0 p-3">
                <div className="flex items-center gap-2 px-1 pb-3">
                  <span className={`size-1.5 rounded-full ${stageDot(stage.key)}`} />
                  <h2 className="text-xs font-medium">{stage.label}</h2>
                  <span className="ml-auto font-mono text-2xs text-ink-3" data-numeric>
                    {files.length}
                  </span>
                </div>

                <div className="space-y-2">
                  {files.map((file) => (
                    <FileCard key={file.id} file={file} />
                  ))}
                </div>
              </section>
            );
          })}
        </div>
      )}
    </>
  );
}

function FileCard({ file }: { file: FileRecord }) {
  return (
    <Link
      href={`/dossiers/${file.id}`}
      className="block rounded-lg border border-line bg-surface-1 p-3 transition-colors duration-[120ms] hover:border-line-strong hover:bg-surface-2"
    >
      <p className="font-mono text-2xs text-ink-3">{file.reference}</p>
      <p className="mt-1 text-xs font-medium">{file.title}</p>

      {file.tags && file.tags.length > 0 && (
        <div className="mt-2 flex flex-wrap gap-1">
          {file.tags.map((tag) => (
            <span
              key={tag}
              className="rounded border border-line px-1.5 py-0.5 text-2xs text-ink-3"
            >
              {tag}
            </span>
          ))}
        </div>
      )}

      <div className="mt-3 flex items-center justify-between gap-2">
        <span className="font-mono text-2xs text-ink-2" data-numeric>
          {formatEUR(file.priceHT)}
        </span>
        {/* La complétude se lit dès le pipeline : c'est ce qui permet de
            repérer un dossier à trous avant l'audit, et le survol dit quelle
            pièce manque. */}
        <ProofMarks proof={file.proof} />
      </div>
    </Link>
  );
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center px-6 py-24 text-center">
      <p className="text-sm font-medium">Aucun dossier pour l&apos;instant</p>
      <p className="mt-1.5 max-w-sm text-xs text-ink-2">
        Un dossier réunit un apprenant, une session et toutes les pièces qui prouvent que
        la formation a bien eu lieu. C&apos;est ce que vous exporterez le jour de
        l&apos;audit.
      </p>
    </div>
  );
}

function stageDot(stage: Stage): string {
  switch (stage) {
    case "in_training":
    case "closed":
      return "bg-ok";
    case "agreement":
      return "bg-warn";
    default:
      return "bg-ink-3";
  }
}


function formatEUR(amount: number): string {
  return new Intl.NumberFormat("fr-FR", {
    style: "currency",
    currency: "EUR",
    maximumFractionDigits: 0,
  }).format(amount);
}
