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
