import type { Metadata, Viewport } from "next";
import { Geist, Geist_Mono } from "next/font/google";
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
  themeColor: "#0b0c0f",
  colorScheme: "dark",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html
      lang="fr"
      className={`${geistSans.variable} ${geistMono.variable} h-full antialiased`}
    >
      <body className="flex min-h-full flex-col">{children}</body>
    </html>
  );
}
