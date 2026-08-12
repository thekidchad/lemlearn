"use client";

import { useCallback, useEffect, useRef, useState } from "react";

interface Coverage {
  percent: number;
  watchedMs: number;
  coveredMs: number;
  lastPosMs: number;
  gaps?: [number, number][];
}

/**
 * Lecteur d'un module, avec sa piste de couverture réelle.
 *
 * Le composant déclare l'intervalle qu'il vient de jouer, pas seulement sa
 * position : c'est ce qui permet au serveur de distinguer une lecture continue
 * d'un saut, sans avoir à le deviner. Il ne calcule aucune progression
 * lui-même — c'est le serveur qui décide de ce qui compte, et un client qui
 * s'auto-évalue n'a aucune valeur probante.
 */
export function ModulePlayer({
  beatUrl,
  durationMs,
  initial,
  heartbeatMs = 5000,
}: {
  beatUrl: string;
  durationMs: number;
  initial: Coverage;
  heartbeatMs?: number;
}) {
  const [coverage, setCoverage] = useState(initial);
  const [playing, setPlaying] = useState(false);
  const [refused, setRefused] = useState<string | null>(null);

  // Position simulée : sans source vidéo diffusable, la démonstration avance
  // le curseur à la vitesse réelle. Le protocole de signaux est identique à
  // celui d'un vrai lecteur HLS.
  const position = useRef(initial.lastPosMs);
  const segmentStart = useRef(initial.lastPosMs);

  const send = useCallback(
    async (fromMs: number, toMs: number) => {
      if (toMs <= fromMs) return;
      try {
        const response = await fetch(beatUrl, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            fromMs,
            toMs,
            rate: 1,
            focused: document.visibilityState === "visible",
          }),
        });
        const body = (await response.json()) as Coverage & {
          accepted: boolean;
          reason?: string;
        };
        if (body.accepted) {
          setRefused(null);
          setCoverage((current) => ({ ...current, ...body }));
        } else {
          setRefused(body.reason ?? "signal écarté");
        }
      } catch {
        // Une coupure réseau ne doit pas interrompre la lecture : le signal
        // suivant reprendra l'intervalle manquant.
      }
    },
    [beatUrl],
  );

  useEffect(() => {
    if (!playing) return;

    const tick = setInterval(() => {
      position.current = Math.min(position.current + heartbeatMs, durationMs);
      send(segmentStart.current, position.current);
      segmentStart.current = position.current;

      if (position.current >= durationMs) setPlaying(false);
    }, heartbeatMs);

    return () => clearInterval(tick);
  }, [playing, durationMs, heartbeatMs, send]);

  const segments = 40;
  const covered = Math.round((coverage.percent / 100) * segments);

  return (
    <div className="surface-card overflow-hidden p-0">
      <div className="relative flex aspect-video items-center justify-center bg-surface-2">
        <button
          type="button"
          onClick={() => setPlaying((value) => !value)}
          className="flex size-14 items-center justify-center rounded-full border border-line-strong bg-surface-0/70 backdrop-blur transition-colors duration-[120ms] hover:border-accent"
          aria-label={playing ? "Mettre en pause" : "Lire le module"}
        >
          {playing ? (
            <span className="flex gap-1">
              <span className="block h-4 w-1 rounded-sm bg-ink" />
              <span className="block h-4 w-1 rounded-sm bg-ink" />
            </span>
          ) : (
            <svg viewBox="0 0 16 16" className="ml-0.5 size-5 text-ink" fill="currentColor">
              <path d="M4.5 2.6 13 7.6a.5.5 0 0 1 0 .86L4.5 13.4a.5.5 0 0 1-.75-.43V3.03a.5.5 0 0 1 .75-.43Z" />
            </svg>
          )}
        </button>

        <p className="absolute top-3 right-4 font-mono text-2xs text-ink-3" data-numeric>
          {formatClock(position.current)} / {formatClock(durationMs)}
        </p>
      </div>

      <div className="space-y-2 border-t border-line px-4 pt-3 pb-4">
        <div className="h-1 rounded-full bg-surface-3">
          <div
            className="h-full rounded-full bg-ink-2 transition-[width] duration-300"
            style={{ width: `${Math.min((position.current / durationMs) * 100, 100)}%` }}
          />
        </div>

        {/* Piste de couverture réelle : ce qui a été vu, pas ce qui a été
            traversé. Un passage rejoué n'allume pas deux segments. */}
        <div className="flex gap-px" aria-hidden>
          {Array.from({ length: segments }, (_, i) => (
            <span
              key={i}
              className={`h-1.5 flex-1 rounded-[1px] ${i < covered ? "bg-accent" : "bg-surface-3"}`}
            />
          ))}
        </div>

        <p className="flex items-center justify-between text-2xs text-ink-3">
          <span>Couverture réelle du module</span>
          <span data-numeric>
            {coverage.percent} % · {Math.round(coverage.coveredMs / 60000)} min vues
          </span>
        </p>

        {refused && (
          <p className="rounded-md border border-warn/40 bg-warn/10 px-2.5 py-1.5 text-2xs text-warn">
            Signal écarté : {refused}
          </p>
        )}
      </div>
    </div>
  );
}

function formatClock(ms: number): string {
  const total = Math.floor(ms / 1000);
  return `${String(Math.floor(total / 60)).padStart(2, "0")}:${String(total % 60).padStart(2, "0")}`;
}
