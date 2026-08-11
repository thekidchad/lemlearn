import { Audience } from "@/components/marketing/audience";
import { Compliance } from "@/components/marketing/compliance";
import { Faq } from "@/components/marketing/faq";
import { Features } from "@/components/marketing/features";
import { Hero } from "@/components/marketing/hero";
import { LearnerPreview } from "@/components/marketing/learner-preview";
import { Pricing } from "@/components/marketing/pricing";
import { ProofChain } from "@/components/marketing/proof-chain";
import { SignatureSection } from "@/components/marketing/signature-section";
import { SiteFooter } from "@/components/marketing/site-footer";
import { SiteHeader } from "@/components/marketing/site-header";

export default function HomePage() {
  return (
    <>
      <SiteHeader />
      <main className="flex-1">
        <Hero />
        <Audience />
        <ProofChain />
        <SignatureSection />
        <Features />
        <LearnerPreview />
        <Compliance />
        <Pricing />
        <Faq />
      </main>
      <SiteFooter />
    </>
  );
}
