import type { Metadata } from "next";
import { Audience } from "@/components/marketing/audience";
import { Compliance } from "@/components/marketing/compliance";
import { CrmSection } from "@/components/marketing/crm-section";
import { Faq } from "@/components/marketing/faq";
import { Features } from "@/components/marketing/features";
import { Hero } from "@/components/marketing/hero";
import { LearnerPreview } from "@/components/marketing/learner-preview";
import { Pricing } from "@/components/marketing/pricing";
import { ProofChain } from "@/components/marketing/proof-chain";
import { SignatureSection } from "@/components/marketing/signature-section";
import { SiteFooter } from "@/components/marketing/site-footer";
import { SiteHeader } from "@/components/marketing/site-header";

// Le produit se nomme ici, et nulle part ailleurs : c'est la seule page qui
// parle de lemlearn à quelqu'un qui cherche lemlearn.
export const metadata: Metadata = {
  title: "lemlearn — le CRM des organismes de formation",
  description:
    "CRM, LMS vidéo et chaîne de preuve horodatée dans un seul outil. Signature électronique intégrée, émargement numérique, exports Qualiopi en un clic.",
  openGraph: {
    type: "website",
    locale: "fr_FR",
    siteName: "lemlearn",
    title: "lemlearn — le CRM des organismes de formation",
  },
};

/**
 * Ordre délibéré : on montre d'abord l'outil de travail quotidien (CRM, puis
 * formations et espace apprenant), et seulement ensuite ce qui le distingue
 * (signature, chaîne de preuve, conformité). Un visiteur qui cherche un CRM de
 * formation doit se reconnaître avant qu'on lui parle d'audit.
 */
export default function HomePage() {
  return (
    <>
      <SiteHeader />
      <main className="flex-1">
        <Hero />
        <Audience />
        <CrmSection />
        <LearnerPreview />
        <SignatureSection />
        <ProofChain />
        <Features />
        <Compliance />
        <Pricing />
        <Faq />
      </main>
      <SiteFooter />
    </>
  );
}
