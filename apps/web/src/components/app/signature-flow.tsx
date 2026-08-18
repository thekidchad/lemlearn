"use client";

import { useCallback, useEffect, useRef, useState } from "react";

/** Mention manuscrite exigée : la recopier est un acte de consentement. */
const MENTION = "Lu et approuvé, bon pour accord";

type Step = "read" | "code" | "sign" | "done";

interface Receipt {
  reference: string;
  signedAt: string;
  sealedSha256: string;
  timestampTsa: boolean;
}

/**
 * Parcours de signature, du document au récépissé.
 *
 * L'ordre des étapes n'est pas cosmétique : lire, puis prouver qu'on est bien
 * le destinataire du courriel, puis consentir de sa main. C'est cet
 * enchaînement — et sa trace horodatée — qui distingue une signature
 * électronique d'une case à cocher.
 */
export function SignatureFlow({
  base,
  reference,
  signerHint,
  sha256,
  alreadySigned,
}: {
  base: string;
  reference: string;
  signerName: string;
  signerHint: string;
  sha256: string;
  alreadySigned: boolean;
}) {
  const [step, setStep] = useState<Step>(alreadySigned ? "done" : "read");
  const [consented, setConsented] = useState(false);
  const [otp, setOtp] = useState("");
  const [mention, setMention] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [receipt, setReceipt] = useState<Receipt | null>(null);
  const [sentTo, setSentTo] = useState<string | null>(null);

  const canvas = useRef<HTMLCanvasElement | null>(null);
  const strokes = useRef(0);
  const startedAt = useRef(0);
  const drawing = useRef(false);
  const [hasDrawing, setHasDrawing] = useState(false);

  const post = useCallback(
    async <T,>(path: string, body?: unknown): Promise<T> => {
      const response = await fetch(`${base}/${path}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        ...(body ? { body: JSON.stringify(body) } : {}),
      });
      const parsed = (await response.json()) as T & { error?: string };
      if (!response.ok) throw new Error(parsed.error ?? "opération impossible");
      return parsed;
    },
    [base],
  );

  const requestCode = async () => {
    setBusy(true);
    setError(null);
    try {
      const { sentTo } = await post<{ sentTo: string }>("otp");
      setSentTo(sentTo);
      setStep("code");
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "envoi impossible");
    } finally {
      setBusy(false);
    }
  };

  // --- Tracé manuscrit -----------------------------------------------------
  // Le canvas est dimensionné en pixels physiques : un tracé rendu en points
  // CSS sur un écran à haute densité arrive flou sur le PDF, et c'est ce PDF
  // qu'un juge regarderait.
  useEffect(() => {
    const element = canvas.current;
    if (!element || step !== "sign") return;

    const ratio = window.devicePixelRatio || 1;
    const rect = element.getBoundingClientRect();
    element.width = rect.width * ratio;
    element.height = rect.height * ratio;

    const context = element.getContext("2d");
    if (!context) return;
    context.scale(ratio, ratio);
    context.lineWidth = 2.2;
    context.lineCap = "round";
    context.lineJoin = "round";
    context.strokeStyle = "#10131a";
  }, [step]);

  const point = (event: React.PointerEvent<HTMLCanvasElement>) => {
    const rect = event.currentTarget.getBoundingClientRect();
    return { x: event.clientX - rect.left, y: event.clientY - rect.top };
  };

  const startStroke = (event: React.PointerEvent<HTMLCanvasElement>) => {
    const context = canvas.current?.getContext("2d");
    if (!context) return;
    event.currentTarget.setPointerCapture(event.pointerId);
    drawing.current = true;
    strokes.current += 1;
    if (startedAt.current === 0) startedAt.current = Date.now();
    const { x, y } = point(event);
    context.beginPath();
    context.moveTo(x, y);
  };

  const continueStroke = (event: React.PointerEvent<HTMLCanvasElement>) => {
    if (!drawing.current) return;
    const context = canvas.current?.getContext("2d");
    if (!context) return;
    const { x, y } = point(event);
    context.lineTo(x, y);
    context.stroke();
    setHasDrawing(true);
  };

  const endStroke = () => {
    drawing.current = false;
  };

  const clear = () => {
    const element = canvas.current;
    const context = element?.getContext("2d");
    if (!element || !context) return;
    context.clearRect(0, 0, element.width, element.height);
    strokes.current = 0;
    startedAt.current = 0;
    setHasDrawing(false);
  };

  const confirm = async () => {
    const element = canvas.current;
    if (!element) return;

    // Le fond est peint en blanc avant l'export : un PNG transparent apposé
    // sur un PDF clair donne un tracé invisible.
    const flat = document.createElement("canvas");
    flat.width = element.width;
    flat.height = element.height;
    const context = flat.getContext("2d");
    if (!context) return;
    context.fillStyle = "#ffffff";
    context.fillRect(0, 0, flat.width, flat.height);
    context.drawImage(element, 0, 0);

    setBusy(true);
    setError(null);
    try {
      setReceipt(
        await post<Receipt>("confirm", {
          otp,
          mention,
          drawingPng: flat.toDataURL("image/png").split(",")[1],
          strokeCount: strokes.current,
          durationMs: startedAt.current ? Date.now() - startedAt.current : 0,
        }),
      );
      setStep("done");
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "signature refusée");
    } finally {
      setBusy(false);
    }
  };

  if (step === "done") {
    return (
      <section className="surface-card mt-6 p-5">
        <h2 className="text-sm font-medium">Document signé</h2>
        <p className="mt-2 text-xs text-ink-2">
          {receipt
            ? `Signé le ${new Date(receipt.signedAt).toLocaleString("fr-FR")}.`
            : "Ce document a déjà été signé."}{" "}
          Il est scellé : toute modification ultérieure invaliderait la
          signature, ce que n&apos;importe quel lecteur PDF sait vérifier.
        </p>

        {receipt && (
          <dl className="mt-4 space-y-2 border-t border-line pt-4 text-2xs">
            <Row label="Référence" value={receipt.reference} />
            <Row label="Empreinte SHA-256 du document scellé" value={receipt.sealedSha256} mono />
            <Row
              label="Horodatage"
              value={
                receipt.timestampTsa
                  ? "jeton d'un tiers horodateur (RFC 3161)"
                  : "heure du serveur, sans jeton de tiers"
              }
            />
          </dl>
        )}

        <a
          href={`${base}/sealed`}
          className="mt-5 inline-flex h-10 items-center rounded-lg border border-line-strong px-4 text-xs font-medium hover:border-accent"
        >
          Télécharger le document signé
        </a>
      </section>
    );
  }

  return (
    <div className="mt-6 space-y-4">
      <section className="surface-card overflow-hidden p-0">
        <div className="flex items-center justify-between border-b border-line px-4 py-3">
          <p className="text-xs font-medium">Document à signer</p>
          <a
            href={`${base}/document`}
            target="_blank"
            rel="noreferrer"
            className="text-2xs text-ink-3 hover:text-ink"
          >
            Ouvrir en plein écran
          </a>
        </div>
        {/* Le document est affiché entier, pas résumé : consentir à un
            extrait n'est pas consentir. */}
        <iframe
          src={`${base}/document`}
          title={`Document ${reference}`}
          className="h-[60vh] w-full bg-white"
        />
        <p className="border-t border-line px-4 py-2.5 font-mono text-2xs break-all text-ink-3">
          SHA-256 : {sha256}
        </p>
      </section>

      {step === "read" && (
        <section className="surface-card p-5">
          <label className="flex cursor-pointer items-start gap-2.5 text-xs text-ink-2">
            <input
              type="checkbox"
              checked={consented}
              onChange={(event) => setConsented(event.target.checked)}
              className="mt-0.5 size-4 accent-[var(--color-accent)]"
            />
            <span>
              J&apos;ai lu l&apos;intégralité du document et j&apos;accepte de le
              signer électroniquement. Je comprends que ma signature a la même
              valeur qu&apos;une signature manuscrite.
            </span>
          </label>

          <button
            type="button"
            onClick={requestCode}
            disabled={!consented || busy}
            className="mt-4 h-11 w-full rounded-lg bg-accent text-sm font-medium text-white transition-colors duration-[120ms] hover:bg-accent-hover disabled:opacity-50"
          >
            {busy ? "Envoi du code…" : `Recevoir un code à ${signerHint}`}
          </button>
        </section>
      )}

      {step === "code" && (
        <section className="surface-card p-5">
          <h2 className="text-sm font-medium">Code de confirmation</h2>
          <p className="mt-1.5 text-xs text-ink-2">
            Un code à six chiffres vient d&apos;être envoyé à {sentTo ?? signerHint}.
            Il est valable dix minutes.
          </p>
          <input
            value={otp}
            onChange={(event) => setOtp(event.target.value.replace(/\D/g, "").slice(0, 6))}
            inputMode="numeric"
            autoComplete="one-time-code"
            placeholder="000000"
            className="mt-4 h-12 w-full rounded-lg border border-line bg-surface-0 text-center font-mono text-lg tracking-[0.4em] outline-none focus:border-accent"
            data-numeric
          />
          <button
            type="button"
            onClick={() => setStep("sign")}
            disabled={otp.length !== 6}
            className="mt-4 h-11 w-full rounded-lg bg-accent text-sm font-medium text-white transition-colors duration-[120ms] hover:bg-accent-hover disabled:opacity-50"
          >
            Continuer
          </button>
          <button
            type="button"
            onClick={requestCode}
            disabled={busy}
            className="mt-2 h-9 w-full text-2xs text-ink-3 hover:text-ink"
          >
            Renvoyer un code
          </button>
        </section>
      )}

      {step === "sign" && (
        <section className="surface-card p-5">
          <h2 className="text-sm font-medium">Mention et signature</h2>
          <p className="mt-1.5 text-xs text-ink-2">
            Recopiez la mention, puis signez dans le cadre.
          </p>

          <p className="mt-4 rounded-lg border border-line bg-surface-2 px-3 py-2 text-xs text-ink-2">
            {MENTION}
          </p>
          <input
            value={mention}
            onChange={(event) => setMention(event.target.value)}
            placeholder="Recopiez la mention ci-dessus"
            className="mt-2 h-11 w-full rounded-lg border border-line bg-surface-0 px-3 text-sm outline-none focus:border-accent"
          />

          <div className="mt-4 overflow-hidden rounded-lg border border-line bg-white">
            <canvas
              ref={canvas}
              onPointerDown={startStroke}
              onPointerMove={continueStroke}
              onPointerUp={endStroke}
              onPointerLeave={endStroke}
              className="h-40 w-full touch-none"
            />
          </div>
          <button
            type="button"
            onClick={clear}
            className="mt-2 text-2xs text-ink-3 hover:text-ink"
          >
            Effacer le tracé
          </button>

          <button
            type="button"
            onClick={confirm}
            disabled={busy || !hasDrawing || mention.trim().toLowerCase() !== MENTION.toLowerCase()}
            className="mt-4 h-11 w-full rounded-lg bg-accent text-sm font-medium text-white transition-colors duration-[120ms] hover:bg-accent-hover disabled:opacity-50"
          >
            {busy ? "Scellement en cours…" : "Signer le document"}
          </button>
          <p className="mt-2 text-2xs text-ink-3">
            Votre adresse IP, votre navigateur et l&apos;horodatage de chaque
            étape sont consignés dans le dossier de preuve joint au document.
          </p>
        </section>
      )}

      {error && (
        <p className="rounded-lg border border-danger/40 bg-danger/10 px-3 py-2 text-xs text-danger">
          {error}
        </p>
      )}
    </div>
  );
}

function Row({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex flex-col gap-0.5">
      <dt className="text-ink-3">{label}</dt>
      <dd className={mono ? "font-mono break-all text-ink-2" : "text-ink-2"}>{value}</dd>
    </div>
  );
}
