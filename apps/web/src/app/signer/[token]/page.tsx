import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { SignatureFlow } from "@/components/app/signature-flow";
import { apiFetch, ApiError, type Brand } from "@/lib/api";
import { OrgBrand, brandStyle } from "@/components/brand/org-brand";

export const metadata: Metadata = {
  title: "Signature électronique",
  // Un lien de signature vaut jeton d'accès : il n'a rien à faire dans un
  // index.
  robots: { index: false, follow: false },
};

interface SignRequest {
  reference: string;
  kind: string;
  signerName: string;
  signerHint: string;
  status: string;
  expiresAt: string;
  sha256: string;
  brand: Brand;
}

const KINDS: Record<string, string> = {
  convention: "Convention de formation",
  devis: "Devis",
  contrat: "Contrat de formation professionnelle",
  emargement: "Feuille d'émargement",
  attestation: "Attestation de fin de formation",
};

export default async function SignerPage({ params }: PageProps<"/signer/[token]">) {
  const { token } = await params;

  let request: SignRequest;
  try {
    request = await apiFetch<SignRequest>(`/v1/sign/${encodeURIComponent(token)}`);
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) notFound();
    if (error instanceof ApiError) {
      return (
        <main className="mx-auto flex min-h-dvh max-w-md flex-col justify-center px-5">
          <h1 className="text-lg font-semibold tracking-[-0.03em]">
            Ce lien n&apos;est plus utilisable
          </h1>
          <p className="mt-2 text-sm text-ink-2">{error.message}</p>
          <p className="mt-4 text-2xs text-ink-3">
            Demandez à l&apos;organisme de formation de vous en envoyer un
            nouveau : un lien de signature est à usage unique et à durée limitée,
            c&apos;est ce qui lui donne sa valeur.
          </p>
        </main>
      );
    }
    throw error;
  }

  return (
    // Le signataire n'a de relation qu'avec son organisme de formation. C'est
    // la page la plus engageante du produit — on y appose une signature — et
    // y voir une marque inconnue est le meilleur moyen de faire abandonner.
    <main className="mx-auto min-h-dvh max-w-xl px-5 py-8" style={brandStyle(request.brand)}>
      <div className="mb-8">
        <OrgBrand brand={request.brand} />
      </div>
      <p className="font-mono text-2xs tracking-wide text-ink-3 uppercase">
        {[KINDS[request.kind] ?? request.kind, request.reference]
          .filter(Boolean)
          .join(" · ")}
      </p>
      <h1 className="mt-2 text-2xl font-semibold tracking-[-0.03em]">
        Signature électronique
      </h1>
      <p className="mt-3 text-sm text-ink-2">
        {request.signerName}, ce document vous engage. Lisez-le entièrement : un
        code de confirmation vous sera envoyé à {request.signerHint}, puis votre
        signature sera horodatée et le document scellé.
      </p>

      <SignatureFlow
        base={`/api/signer/${encodeURIComponent(token)}`}
        reference={request.reference}
        signerName={request.signerName}
        signerHint={request.signerHint}
        sha256={request.sha256}
        alreadySigned={request.status === "signed"}
      />
    </main>
  );
}
