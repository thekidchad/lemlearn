import type { Metadata, Viewport } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import { cookies } from "next/headers";
import { THEME_COOKIE } from "@/lib/theme";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

/**
 * La racine ne nomme pas le produit.
 *
 * Elle habille aussi bien la vitrine que les pages de signature, et celles-ci
 * sont vues par des gens qui ne connaissent que leur organisme de formation —
 * jusque dans l'aperçu d'un lien partagé dans une conversation. Le nom, la
 * description et les métadonnées de partage appartiennent donc à la vitrine,
 * qui les déclare pour elle-même.
 */
export const metadata: Metadata = {
  metadataBase: new URL("https://lemlearn.fr"),
  title: { default: "Formation professionnelle", template: "%s" },
  openGraph: { type: "website", locale: "fr_FR" },
};

export const viewport: Viewport = {
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#fbfbfd" },
    { media: "(prefers-color-scheme: dark)", color: "#0b0c0f" },
  ],
  colorScheme: "light dark",
};

/**
 * Le thème est lu d'un cookie, côté serveur.
 *
 * C'est ce qui évite le clignotement blanc au chargement : la page arrive déjà
 * dans le bon thème, sans script qui corrige après coup. Sans cookie, on ne
 * pose rien et le réglage du système décide.
 */
export default async function RootLayout({ children }: LayoutProps<"/">) {
  const theme = (await cookies()).get(THEME_COOKIE)?.value;

  return (
    <html
      lang="fr"
      data-theme={theme === "light" || theme === "dark" ? theme : undefined}
      className={`${geistSans.variable} ${geistMono.variable} h-full antialiased`}
    >
      <body className="flex min-h-full flex-col">{children}</body>
    </html>
  );
}
