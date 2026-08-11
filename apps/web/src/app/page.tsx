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
