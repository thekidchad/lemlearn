import type { Metadata } from "next";
import Link from "next/link";
import { TemplateEditor } from "@/components/app/template-editor";
import { apiFetch } from "@/lib/api";

export const metadata: Metadata = { title: "Gabarits de courriels" };

export interface Template {
  key: string;
  label: string;
  purpose: string;
  subject: string;
  body: string;
  variables: { name: string; purpose: string; sample: string }[];
  overridden: boolean;
  updatedAt?: string;
  updatedBy?: string;
  defaultSubject: string;
  defaultBody: string;
}

/**
 * Gabarits des courriels transactionnels.
 *
 * Ils sont modifiables sans redéploiement : une formule maladroite dans un
 * message envoyé à des signataires ne doit pas attendre la prochaine version
 * pour être corrigée.
 */
export default async function TemplatesPage({ searchParams }: PageProps<"/admin/gabarits">) {
  const params = await searchParams;
  const selected = typeof params.key === "string" ? params.key : undefined;

  const { templates } = await apiFetch<{ templates: Template[] }>("/v1/admin/gabarits");
  const current = templates.find((template) => template.key === selected) ?? templates[0];

  return (
    <>
      <header className="flex h-14 items-center gap-3 border-b border-line px-6">
        <Link href="/admin" className="text-xs text-ink-3 hover:text-ink">
          Organisations
        </Link>
        <span className="text-ink-3">/</span>
        <h1 className="text-sm font-medium">Gabarits de courriels</h1>
        <Link
          href="/admin/journal/courriels"
          className="ml-auto text-2xs text-ink-3 underline hover:text-ink"
        >
          Journal
        </Link>
      </header>

      <div className="flex min-h-[calc(100vh-3.5rem)]">
        <nav className="w-64 shrink-0 border-r border-line p-3">
          {templates.map((template) => (
            <Link
              key={template.key}
              href={`/admin/gabarits?key=${template.key}`}
              aria-current={template.key === current?.key ? "page" : undefined}
              className={`block rounded-lg px-3 py-2.5 transition-colors duration-[120ms] ${
                template.key === current?.key
                  ? "bg-surface-2 text-ink"
                  : "text-ink-2 hover:bg-surface-2/60 hover:text-ink"
              }`}
            >
              <span className="flex items-center justify-between gap-2">
                <span className="truncate text-xs font-medium">{template.label}</span>
                {template.overridden && (
                  <span className="shrink-0 rounded bg-accent-dim px-1.5 py-0.5 text-2xs text-accent-ink">
                    modifié
                  </span>
                )}
              </span>
              <span className="mt-0.5 block font-mono text-2xs text-ink-3">{template.key}</span>
            </Link>
          ))}
        </nav>

        <div className="min-w-0 flex-1 p-6">
          {/* La clé remonte l'éditeur quand on change de message : ses champs
              repartent du gabarit choisi, sans effet de synchronisation. */}
          {current ? (
            <TemplateEditor key={current.key} template={current} />
          ) : (
            <p>Aucun gabarit.</p>
          )}
        </div>
      </div>
    </>
  );
}
