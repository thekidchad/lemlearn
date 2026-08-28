"use client";

import { useRouter } from "next/navigation";
import { useRef, useState } from "react";
import type { Brand } from "@/lib/api";
import { OrgMark } from "@/components/brand/org-brand";

/** Ce que l'API accepte, et ce que les messageries savent afficher. */
const ACCEPTED = "image/svg+xml,image/png,image/jpeg";

/**
 * Six teintes de départ, pour qu'un organisme sans charte graphique n'ait pas
 * à ouvrir un sélecteur de couleur. Elles sont toutes assez foncées pour
 * porter du texte blanc, et assez distinctes pour qu'on ne les confonde pas.
 */
const PRESETS = ["#6644E8", "#0A7C5A", "#B4472A", "#1D4ED8", "#8B2F62", "#3F4756"];

const THEMES: { valeur: string; label: string; aide: string }[] = [
  { valeur: "system", label: "Automatique", aide: "suit le réglage de l'appareil" },
  { valeur: "light", label: "Clair", aide: "fond clair par défaut" },
  { valeur: "dark", label: "Sombre", aide: "fond sombre par défaut" },
];

interface Saved {
  brand: {
    name?: string;
    logoKey?: string;
    accent?: string;
    supportEmail?: string;
    theme?: string;
  };
  resolved: Brand;
  orgName: string;
}

/**
 * Réglage de l'identité visible d'un organisme.
 *
 * Le même formulaire sert à l'organisme pour lui-même et à l'équipe lemlearn
 * pour n'importe lequel de ses clients : `base` change, rien d'autre. C'est ce
 * qui permet d'ouvrir un client en deux minutes sans rien lui demander.
 *
 * L'aperçu est immédiat et local : choisir une couleur puis attendre un
 * aller-retour serveur pour la voir rendrait le réglage pénible, alors que
 * c'est justement le moment où l'on tâtonne.
 */
export function BrandForm({ base, initial }: { base: string; initial: Saved }) {
  const router = useRouter();
  const input = useRef<HTMLInputElement | null>(null);

  const [name, setName] = useState(initial.brand.name ?? "");
  const [accent, setAccent] = useState(initial.brand.accent ?? initial.resolved.accent);
  const [supportEmail, setSupportEmail] = useState(initial.brand.supportEmail ?? "");
  const [theme, setTheme] = useState(initial.brand.theme ?? "system");
  const [logoKey, setLogoKey] = useState(initial.brand.logoKey ?? "");
  const [logoUrl, setLogoUrl] = useState(initial.resolved.logoUrl ?? "");
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  const preview: Brand = {
    name: name.trim() || initial.orgName,
    logoUrl,
    monogram: monogram(name.trim() || initial.orgName),
    accent,
    accentInk: inkOn(accent),
    theme,
  };

  const upload = async (file: File) => {
    setError(null);
    setSaved(false);
    setBusy("dépôt");
    try {
      const reserve = await fetch(`${base}/logo`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ contentType: file.type }),
      });
      const reserved = (await reserve.json()) as {
        uploadUrl?: string;
        key?: string;
        error?: string;
      };
      if (!reserve.ok || !reserved.uploadUrl || !reserved.key) {
        throw new Error(reserved.error ?? "dépôt refusé");
      }

      const put = await fetch(reserved.uploadUrl, {
        method: "PUT",
        headers: { "Content-Type": file.type },
        body: file,
      });
      if (!put.ok) throw new Error(`le dépôt a échoué (${put.status})`);

      setLogoKey(reserved.key);
      // L'aperçu lit l'objet fraîchement déposé. L'horodatage force le
      // navigateur à le recharger : sans lui, remplacer un logo laisserait
      // l'ancien à l'écran, sous la même adresse.
      setLogoUrl(URL.createObjectURL(file));
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "dépôt impossible");
    } finally {
      setBusy(null);
    }
  };

  const save = async () => {
    setError(null);
    setBusy("enregistrement");
    try {
      const response = await fetch(base, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: name.trim(),
          accent,
          theme,
          supportEmail: supportEmail.trim(),
          logoKey,
        }),
      });
      const body = (await response.json()) as { error?: string; resolved?: Brand };
      if (!response.ok) throw new Error(body.error ?? "enregistrement refusé");
      if (body.resolved?.logoUrl) setLogoUrl(body.resolved.logoUrl);
      setSaved(true);
      router.refresh();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "enregistrement impossible");
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="grid gap-5 lg:grid-cols-[1fr_20rem]">
      <section className="surface-card p-5">
        <h2 className="text-sm font-medium">Identité de l&apos;organisme</h2>
        <p className="mt-1.5 text-xs text-ink-2">
          Ce que voient vos apprenants et vos signataires — dans l&apos;application, dans
          les courriels et sur les pages de signature. Rien de tout cela ne porte
          notre nom.
        </p>

        <label className="mt-5 block">
          <span className="eyebrow">Nom affiché</span>
          <input
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder={initial.orgName}
            maxLength={60}
            className="field mt-1.5"
          />
          <span className="mt-1 block text-2xs text-ink-3">
            Vide, votre raison sociale est employée : {initial.orgName}.
          </span>
        </label>

        <div className="mt-5">
          <span className="eyebrow">Logo</span>
          <div className="mt-1.5 flex flex-wrap items-center gap-3">
            <input
              ref={input}
              type="file"
              accept={ACCEPTED}
              hidden
              onChange={(event) => {
                const file = event.target.files?.[0];
                if (file) void upload(file);
                event.target.value = "";
              }}
            />
            <button
              type="button"
              className="btn-secondary"
              disabled={busy !== null}
              onClick={() => input.current?.click()}
            >
              {logoKey ? "Remplacer" : "Déposer un logo"}
            </button>
            {logoKey && (
              <button
                type="button"
                className="btn-ghost"
                disabled={busy !== null}
                onClick={() => {
                  setLogoKey("");
                  setLogoUrl("");
                  setSaved(false);
                }}
              >
                Retirer
              </button>
            )}
            <span className="text-2xs text-ink-3">SVG, PNG ou JPEG</span>
          </div>
          <p className="mt-1.5 text-2xs text-ink-3">
            Sans logo, vos initiales s&apos;affichent dans votre couleur. Préférez une
            image à fond transparent : elle doit tenir sur un fond clair comme sur
            un fond sombre.
          </p>
        </div>

        <div className="mt-5">
          <span className="eyebrow">Couleur d&apos;accent</span>
          <div className="mt-1.5 flex flex-wrap items-center gap-2">
            {PRESETS.map((teinte) => (
              <button
                key={teinte}
                type="button"
                aria-label={`Choisir ${teinte}`}
                aria-pressed={accent.toUpperCase() === teinte}
                onClick={() => {
                  setAccent(teinte);
                  setSaved(false);
                }}
                style={{ background: teinte }}
                className={`size-7 rounded-md ring-offset-2 ring-offset-surface-1 transition ${
                  accent.toUpperCase() === teinte ? "ring-2 ring-ink" : "ring-1 ring-line"
                }`}
              />
            ))}
            <input
              type="color"
              value={accent}
              onChange={(event) => {
                setAccent(event.target.value.toUpperCase());
                setSaved(false);
              }}
              aria-label="Couleur personnalisée"
              className="size-7 cursor-pointer rounded-md border border-line bg-transparent p-0.5"
            />
            <span className="font-mono text-2xs text-ink-3">{accent.toUpperCase()}</span>
          </div>
        </div>

        <div className="mt-5">
          <span className="eyebrow">Thème par défaut</span>
          <p className="mt-1 text-2xs text-ink-3">
            Ce que voit quelqu&apos;un qui arrive pour la première fois. Chacun peut
            ensuite basculer : ce réglage propose, il n&apos;impose pas.
          </p>
          <div className="mt-2 flex flex-wrap gap-2">
            {THEMES.map((option) => (
              <button
                key={option.valeur}
                type="button"
                aria-pressed={theme === option.valeur}
                onClick={() => {
                  setTheme(option.valeur);
                  setSaved(false);
                }}
                className={`rounded-lg border px-3 py-2 text-left transition ${
                  theme === option.valeur
                    ? "border-accent bg-accent-dim"
                    : "border-line hover:border-line-strong"
                }`}
              >
                <span className="block text-xs font-medium">{option.label}</span>
                <span className="block text-2xs text-ink-3">{option.aide}</span>
              </button>
            ))}
          </div>
        </div>

        <label className="mt-5 block">
          <span className="eyebrow">Adresse de contact</span>
          <input
            value={supportEmail}
            onChange={(event) => setSupportEmail(event.target.value)}
            placeholder="formation@votre-organisme.fr"
            className="field mt-1.5"
          />
          <span className="mt-1 block text-2xs text-ink-3">
            Affichée à vos apprenants lorsqu&apos;ils cherchent à vous joindre.
          </span>
        </label>

        {error && <p className="mt-4 text-xs text-danger">{error}</p>}

        <div className="mt-6 flex items-center gap-3">
          <button type="button" className="btn-primary" disabled={busy !== null} onClick={save}>
            {busy === "enregistrement" ? "Enregistrement…" : "Enregistrer"}
          </button>
          {busy === "dépôt" && <span className="text-xs text-ink-2">Dépôt du logo…</span>}
          {saved && busy === null && (
            <span className="text-xs text-ok">Enregistré. Vos apprenants le voient déjà.</span>
          )}
        </div>
      </section>

      <aside className="surface-card p-5" style={{ ["--color-accent" as string]: accent }}>
        <h2 className="text-sm font-medium">Aperçu</h2>
        <p className="mt-1.5 text-xs text-ink-2">Ce que voit un apprenant.</p>

        <div className="mt-4 rounded-lg border border-line p-3">
          <OrgBrandPreview brand={preview} />
        </div>

        <div className="mt-4 rounded-lg border border-line p-3">
          <p className="text-2xs text-ink-3">Bouton d&apos;un courriel</p>
          <span
            style={{ background: accent, color: preview.accentInk }}
            className="mt-2 inline-block rounded-lg px-3.5 py-2 text-xs font-medium"
          >
            Lire et signer le document
          </span>
        </div>
      </aside>
    </div>
  );
}

function OrgBrandPreview({ brand }: { brand: Brand }) {
  return (
    <span className="inline-flex min-w-0 items-center gap-2.5">
      <OrgMark brand={brand} />
      <span className="truncate text-[0.9375rem] font-semibold tracking-[-0.045em] text-ink">
        {brand.name}
      </span>
    </span>
  );
}

/**
 * Le monogramme, reproduit ici pour l'aperçu seul.
 *
 * L'API reste la référence — c'est elle qui le calcule pour les courriels et
 * pour tous les écrans. Attendre un aller-retour pour voir deux lettres
 * changer pendant qu'on tape rendrait le réglage désagréable.
 */
function monogram(nom: string): string {
  const liaison = new Set(["de", "du", "des", "la", "le", "les", "et", "d", "l", "en"]);
  const mots = nom
    .split(/[^\p{L}\p{N}]+/u)
    .filter(Boolean)
    .filter((mot) => !liaison.has(mot.toLowerCase()));
  if (mots.length === 0) return "OF";
  if (mots.length === 1) return [...mots[0]].slice(0, 2).join("").toUpperCase();
  return (mots[0][0] + mots[1][0]).toUpperCase();
}

/** Même seuil de luminance que l'API, pour que l'aperçu ne mente pas. */
function inkOn(hex: string): string {
  const valeur = /^#[0-9a-f]{6}$/i.test(hex) ? hex : "#000000";
  const r = parseInt(valeur.slice(1, 3), 16);
  const g = parseInt(valeur.slice(3, 5), 16);
  const b = parseInt(valeur.slice(5, 7), 16);
  return (0.299 * r + 0.587 * g + 0.114 * b) / 255 > 0.62 ? "#10131A" : "#FFFFFF";
}
