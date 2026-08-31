import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  /**
   * Sortie autonome : Next assemble un serveur complet avec ses seules
   * dépendances utiles, quelques dizaines de mégaoctets au lieu du
   * `node_modules` entier. C'est ce qui rend l'hébergement en Lambda possible
   * — le paquet doit tenir sous les 250 Mo décompressés.
   */
  output: "standalone",

  // Le serveur tourne dans une Lambda derrière CloudFront : c'est CloudFront
  // qui voit l'adresse du visiteur, et Next doit lire l'en-tête transmis
  // plutôt que l'adresse de la passerelle.
  poweredByHeader: false,

  /**
   * « Contacts » était du jargon de CRM qui ne disait rien du métier : les
   * trois natures ont désormais chacune leur écran, nommé comme ce qu'il
   * contient. Le code du travail dit « stagiaire ».
   *
   * La redirection vit ici plutôt que dans une page : rendue par un composant
   * serveur, elle arrivait après que la coque a été envoyée, donc sans en-tête
   * `Location`. Un navigateur suivait quand même, mais rien d'autre.
   */
  async redirects() {
    return [
      {
        // Le journal des envois est devenu un onglet du journal : l'adresse
        // d'hier reste bonne, elle mène simplement au bon onglet.
        source: "/admin/emails",
        destination: "/admin/journal/courriels",
        permanent: true,
      },
      {
        source: "/contacts",
        has: [{ type: "query", key: "kind", value: "company" }],
        destination: "/entreprises",
        permanent: true,
      },
      {
        source: "/contacts",
        has: [{ type: "query", key: "kind", value: "funder" }],
        destination: "/financeurs",
        permanent: true,
      },
      { source: "/contacts", destination: "/stagiaires", permanent: true },
      { source: "/contacts/:id", destination: "/stagiaires/:id", permanent: true },
    ];
  },

  images: {
    /**
     * L'optimiseur d'images de Next s'appuie sur `sharp`, dont le binaire est
     * compilé pour la machine qui a assemblé le paquet — celui de macOS ne
     * démarre pas sur Lambda, et l'embarquer coûterait trente mégaoctets pour
     * rien. Les images du site sont déjà dimensionnées à la main et servies
     * par CloudFront avec un cache long : les faire redimensionner à chaque
     * démarrage à froid n'apporterait rien.
     */
    unoptimized: process.env.LEMLEARN_BUILD === "lambda",
  },
};

export default nextConfig;
