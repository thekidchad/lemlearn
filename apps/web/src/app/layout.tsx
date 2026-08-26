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

export const metadata: Metadata = {
  metadataBase: new URL("https://lemlearn.fr"),
  title: {
    default: "lemlearn — le CRM des organismes de formation",
    template: "%s · lemlearn",
  },
  description:
    "CRM, LMS vidéo et chaîne de preuve horodatée dans un seul outil. Signature électronique intégrée, émargement numérique, exports Qualiopi en un clic.",
  openGraph: {
    type: "website",
    locale: "fr_FR",
    siteName: "lemlearn",
  },
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
