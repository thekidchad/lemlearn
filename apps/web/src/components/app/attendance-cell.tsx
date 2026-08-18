"use client";

import { useRouter } from "next/navigation";
import { useState, useTransition } from "react";
import { markAttendance } from "@/app/actions/crm";

type Method = "signature" | "connection" | "absent";

const LABELS: Record<Method, string> = {
  signature: "Signé",
  connection: "Connexion",
  absent: "Absent",
};

/**
 * Case d'émargement, modifiable par le formateur.
 *
 * Une case vide n'est pas une absence : c'est un créneau non traité, et les
 * deux se distinguent à l'écran comme au journal. Marquer une absence est un
 * acte, pas un oubli.
 */
export function AttendanceCell({
  sessionId,
  slotId,
  contactId,
  method,
  coveragePercent,
}: {
  sessionId: string;
  slotId: string;
  contactId: string;
  method?: Method;
  coveragePercent?: number;
}) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [error, setError] = useState<string | null>(null);

  const style = !method
    ? "bg-surface-3 text-ink-3"
    : method === "absent"
      ? "bg-bad/20 text-bad"
      : method === "connection"
        ? "bg-accent/20 text-accent-ink"
        : "bg-ok/20 text-ok";

  const label = !method
    ? "—"
    : method === "connection"
      ? `${coveragePercent ?? 0} %`
      : LABELS[method];

  return (
    <span className="inline-flex items-center gap-1.5">
      <select
        value={method ?? ""}
        disabled={pending}
        aria-label="État du créneau"
        title={error ?? "Créneau non traité tant qu'aucun état n'est choisi"}
        onChange={(event) => {
          const next = event.target.value as Method;
          startTransition(async () => {
            const result = await markAttendance(sessionId, slotId, contactId, next);
            setError(result.error ?? null);
            if (!result.error) router.refresh();
          });
        }}
        className={`h-6 rounded px-1.5 text-2xs outline-none focus:ring-1 focus:ring-accent ${style}`}
      >
        <option value="" disabled>
          {label}
        </option>
        <option value="signature">Signé</option>
        <option value="connection">Présence par connexion</option>
        <option value="absent">Absent</option>
      </select>
      {error && <span className="text-2xs text-danger">!</span>}
    </span>
  );
}
