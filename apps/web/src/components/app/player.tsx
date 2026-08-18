"use client";

import { useCallback, useEffect, useRef, useState } from "react";

interface Coverage {
  percent: number;
  watchedMs: number;
  coveredMs: number;
  lastPosMs: number;
  gaps?: [number, number][];
}

interface Playback {
  durationMs: number;
  heartbeatMs: number;
  expiresAt: string;
}

/**
 * Lecteur d'un module, avec sa piste de couverture réelle.
 *
 * Le composant déclare l'intervalle qu'il vient de jouer, pas seulement sa
 * position : c'est ce qui permet au serveur de distinguer une lecture continue
 * d'un saut, sans avoir à le deviner. Il ne calcule aucune progression
 * lui-même — c'est le serveur qui décide de ce qui compte, et un client qui
 * s'auto-évalue n'a aucune valeur probante.
 *
 * Quand le module ne porte pas encore de vidéo, le lecteur bascule sur un
 * curseur simulé qui parle exactement le même protocole de signaux : la chaîne
 * d'assiduité reste démontrable avant que le catalogue ne soit tourné.
 */
export function ModulePlayer({
  beatUrl,
  playbackUrl,
  manifestUrl,
  durationMs,
  initial,
  heartbeatMs = 5000,
}: {
  beatUrl: string;
  playbackUrl: string;
  manifestUrl: string;
  durationMs: number;
  initial: Coverage;
  heartbeatMs?: number;
}) {
  const [coverage, setCoverage] = useState(initial);
  const [playing, setPlaying] = useState(false);
  const [refused, setRefused] = useState<string | null>(null);
  const [source, setSource] = useState<"pending" | "video" | "simulated">("pending");
  const [reason, setReason] = useState<string | null>(null);
  const [beat, setBeat] = useState(heartbeatMs);
  const [position, setPosition] = useState(initial.lastPosMs);
  // La durée déclarée au catalogue sert de repli ; celle du fichier fait foi
  // dès que le lecteur l'a lue.
  const [total, setTotal] = useState(durationMs);

  const video = useRef<HTMLVideoElement | null>(null);
  // Borne inférieure de l'intervalle non encore déclaré. Un saut la replace
  // sans rien déclarer entre les deux : c'est ce qui empêche un `currentTime`
  // poussé à la fin de compter comme du temps suivi.
  const segmentStart = useRef(initial.lastPosMs);
  const cursor = useRef(initial.lastPosMs);

  const send = useCallback(
    async (fromMs: number, toMs: number) => {
      if (toMs <= fromMs) return;
      try {
        const response = await fetch(beatUrl, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            fromMs: Math.round(fromMs),
            toMs: Math.round(toMs),
            rate: video.current?.playbackRate ?? 1,
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

  // --- Ouverture de la séance ---------------------------------------------
  // L'autorisation se demande à l'affichage du lecteur, pas au rendu de la
  // page : elle expire en quinze minutes, et une page laissée ouverte en
  // aurait une périmée.
  useEffect(() => {
    let cancelled = false;
    let cleanup: (() => void) | undefined;

    (async () => {
      let playback: Playback;
      try {
        const response = await fetch(playbackUrl, { method: "POST" });
        const body = (await response.json()) as Playback & { error?: string };
        if (!response.ok) throw new Error(body.error ?? "lecture indisponible");
        playback = body;
      } catch (error) {
        if (cancelled) return;
        setReason(error instanceof Error ? error.message : "lecture indisponible");
        setSource("simulated");
        return;
      }

      if (cancelled) return;
      if (playback.heartbeatMs) setBeat(playback.heartbeatMs);

      const element = video.current;
      if (!element) return;

      // Safari lit le HLS nativement ; partout ailleurs il faut hls.js, chargé
      // seulement ici pour ne pas peser sur les pages qui ne lisent rien.
      if (element.canPlayType("application/vnd.apple.mpegurl")) {
        element.src = manifestUrl;
      } else {
        const { default: Hls } = await import("hls.js");
        if (cancelled || !Hls.isSupported()) {
          setReason("ce navigateur ne sait pas lire le format de diffusion");
          setSource("simulated");
          return;
        }
        const hls = new Hls({ enableWorker: true });
        hls.loadSource(manifestUrl);
        hls.attachMedia(element);
        cleanup = () => hls.destroy();
      }

      if (initial.lastPosMs > 0) {
        // Reprendre où l'apprenant s'était arrêté, sans compter la reprise
        // comme du visionnage.
        element.currentTime = initial.lastPosMs / 1000;
        segmentStart.current = initial.lastPosMs;
      }
      setSource("video");
    })();

    return () => {
      cancelled = true;
      cleanup?.();
    };
  }, [playbackUrl, manifestUrl, initial.lastPosMs]);

  // --- Signaux -------------------------------------------------------------
  useEffect(() => {
    if (!playing) return;

    const tick = setInterval(() => {
      const element = video.current;
      const at =
        source === "video" && element
          ? element.currentTime * 1000
          : Math.min(cursor.current + beat, durationMs);

      cursor.current = at;
      setPosition(at);
      send(segmentStart.current, at);
      segmentStart.current = at;

      if (source !== "video" && at >= durationMs) setPlaying(false);
    }, beat);

    return () => clearInterval(tick);
  }, [playing, source, durationMs, beat, send]);

  // Un saut ne se déclare pas : il déplace la borne. Le serveur le
  // détecterait de toute façon, mais autant ne pas lui envoyer un intervalle
  // que personne n'a regardé.
  const onSeeked = useCallback(() => {
    const element = video.current;
    if (!element) return;
    segmentStart.current = element.currentTime * 1000;
    cursor.current = segmentStart.current;
    setPosition(segmentStart.current);
  }, []);

  // Quitter la page en cours de lecture ne doit pas perdre la dernière
  // minute : le navigateur laisse passer une requête si elle est brève.
  useEffect(() => {
    const flush = () => {
      const at = video.current ? video.current.currentTime * 1000 : cursor.current;
      if (at > segmentStart.current) {
        navigator.sendBeacon?.(
          beatUrl,
          new Blob(
            [
              JSON.stringify({
                fromMs: Math.round(segmentStart.current),
                toMs: Math.round(at),
                rate: 1,
                focused: false,
              }),
            ],
            { type: "application/json" },
          ),
        );
        segmentStart.current = at;
      }
    };
    document.addEventListener("visibilitychange", flush);
    window.addEventListener("pagehide", flush);
    return () => {
      document.removeEventListener("visibilitychange", flush);
      window.removeEventListener("pagehide", flush);
    };
  }, [beatUrl]);

  const segments = 40;
  const covered = Math.round((coverage.percent / 100) * segments);

  const toggle = () => {
    const element = video.current;
    if (source === "video" && element) {
      if (element.paused) void element.play();
      else element.pause();
      return;
    }
    setPlaying((value) => !value);
  };

  return (
    <div className="surface-card overflow-hidden p-0">
      <div className="relative flex aspect-video items-center justify-center bg-surface-2">
        <video
          ref={video}
          className={`size-full ${source === "video" ? "" : "hidden"}`}
          playsInline
          controls
          controlsList="nodownload"
          onPlay={() => setPlaying(true)}
          onPause={() => setPlaying(false)}
          onEnded={() => setPlaying(false)}
          onSeeked={onSeeked}
          onTimeUpdate={(event) => setPosition(event.currentTarget.currentTime * 1000)}
          onLoadedMetadata={(event) => {
            const seconds = event.currentTarget.duration;
            if (Number.isFinite(seconds) && seconds > 0) setTotal(seconds * 1000);
          }}
        />

        {source !== "video" && (
          <button
            type="button"
            onClick={toggle}
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
        )}

        {source !== "video" && (
          <p className="absolute top-3 right-4 font-mono text-2xs text-ink-3" data-numeric>
            {formatClock(position)} / {formatClock(total)}
          </p>
        )}
      </div>

      <div className="space-y-2 border-t border-line px-4 pt-3 pb-4">
        {source !== "video" && (
          <div className="h-1 rounded-full bg-surface-3">
            <div
              className="h-full rounded-full bg-ink-2 transition-[width] duration-300"
              style={{ width: `${Math.min((position / total) * 100, 100)}%` }}
            />
          </div>
        )}

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

        {source === "simulated" && reason && (
          <p className="text-2xs text-ink-3">
            {/* Dire pourquoi il n'y a pas d'image, plutôt que d'afficher un
                lecteur qui ne lit rien. */}
            Aucune vidéo diffusable ({reason}) — le curseur ci-dessus simule la
            lecture et produit les mêmes signaux d&apos;assiduité.
          </p>
        )}

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
