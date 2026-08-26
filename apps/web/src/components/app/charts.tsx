/**
 * Graphiques du tableau de bord.
 *
 * Écrits en SVG, sans bibliothèque : trois formes suffisent ici, et une
 * dépendance de graphiques pèse plus lourd que le reste de la page.
 *
 * Une seule teinte porte les données — l'accent — parce qu'aucun de ces
 * graphiques ne compare des identités : ils comparent des grandeurs. Une
 * palette catégorielle ferait croire à des séries distinctes là où il n'y a
 * qu'une quantité qui monte ou descend. Le rouge n'y sert que d'état, jamais de
 * série, et toujours accompagné d'un libellé.
 */

const SURFACE = "var(--color-surface-1, #121318)";

/** Aire d'évolution — une seule série, donc aucune légende à porter. */
export function TrendArea({
  points,
  label,
  format = (value: number) => String(value),
}: {
  points: { x: string; y: number }[];
  label: string;
  format?: (value: number) => string;
}) {
  const width = 640;
  const height = 160;
  const padding = { top: 12, right: 12, bottom: 22, left: 28 };
  const max = Math.max(1, ...points.map((point) => point.y));

  const plotWidth = width - padding.left - padding.right;
  const plotHeight = height - padding.top - padding.bottom;
  const stepX = points.length > 1 ? plotWidth / (points.length - 1) : 0;

  const coordinates = points.map((point, index) => ({
    ...point,
    px: padding.left + index * stepX,
    py: padding.top + plotHeight - (point.y / max) * plotHeight,
  }));

  const line = coordinates.map((point) => `${point.px},${point.py}`).join(" ");
  const area = `${padding.left},${padding.top + plotHeight} ${line} ${padding.left + plotWidth},${
    padding.top + plotHeight
  }`;
  const last = coordinates[coordinates.length - 1];

  return (
    <figure className="m-0">
      <svg
        viewBox={`0 0 ${width} ${height}`}
        className="w-full"
        role="img"
        aria-label={`${label} : ${points.map((point) => `${point.x} ${point.y}`).join(", ")}`}
      >
        {/* Deux repères seulement : une grille dense concurrence les données. */}
        {[0, 0.5, 1].map((ratio) => (
          <line
            key={ratio}
            x1={padding.left}
            x2={padding.left + plotWidth}
            y1={padding.top + plotHeight * ratio}
            y2={padding.top + plotHeight * ratio}
            stroke="var(--color-line, #24262F)"
            strokeWidth="1"
          />
        ))}
        <text x="0" y={padding.top + 4} className="fill-ink-3 text-[9px]" style={{ fontSize: 9 }}>
          {format(max)}
        </text>

        <polygon points={area} fill="var(--color-accent, #7C5CFF)" fillOpacity="0.1" />
        <polyline
          points={line}
          fill="none"
          stroke="var(--color-accent, #7C5CFF)"
          strokeWidth="2"
          strokeLinejoin="round"
          strokeLinecap="round"
        />

        {/* Cibles de survol : plus larges que les points, et invisibles. Un
            point de deux pixels ne s'attrape pas à la souris. */}
        {coordinates.map((point) => (
          <circle key={point.x} cx={point.px} cy={point.py} r="8" fill="transparent">
            <title>{`${point.x} — ${format(point.y)}`}</title>
          </circle>
        ))}

        {/* Un seul point marqué — le dernier — et une seule étiquette : une
            valeur sur chaque point ferait un tableau déguisé en courbe. */}
        {last && (
          <>
            <circle
              cx={last.px}
              cy={last.py}
              r="4"
              fill="var(--color-accent, #7C5CFF)"
              stroke={SURFACE}
              strokeWidth="2"
            />
            <text
              x={last.px - 8}
              y={Math.min(Math.max(last.py + 4, padding.top + 10), padding.top + plotHeight - 4)}
              textAnchor="end"
              className="fill-ink-2"
              style={{ fontSize: 10 }}
            >
              {format(last.y)}
            </text>
          </>
        )}

        {coordinates.length > 0 && (
          <>
            <text x={padding.left} y={height - 6} className="fill-ink-3" style={{ fontSize: 9 }}>
              {coordinates[0].x}
            </text>
            <text
              x={padding.left + plotWidth}
              y={height - 6}
              textAnchor="end"
              className="fill-ink-3"
              style={{ fontSize: 9 }}
            >
              {coordinates[coordinates.length - 1].x}
            </text>
          </>
        )}
      </svg>
    </figure>
  );
}

/** Colonnes — comparaison de grandeurs dans le temps. */
export function Columns({
  bars,
  label,
}: {
  bars: { x: string; y: number }[];
  label: string;
}) {
  const max = Math.max(1, ...bars.map((bar) => bar.y));

  return (
    <figure
      className="m-0 flex h-40 items-end gap-2"
      role="img"
      aria-label={`${label} : ${bars.map((bar) => `${bar.x} ${bar.y}`).join(", ")}`}
    >
      {bars.map((bar) => (
        <div
          key={bar.x}
          title={`${bar.x} — ${bar.y}`}
          className="flex h-full min-w-0 flex-1 flex-col items-center justify-end gap-1.5"
        >
          <span className="font-mono text-2xs text-ink-2" data-numeric>
            {bar.y}
          </span>
          {/* La piste porte la hauteur : un pourcentage dans un parent sans
              hauteur définie s'effondre à zéro, et la colonne disparaît. */}
          <div className="flex w-full max-w-6 flex-1 items-end">
            <div
              className="w-full rounded-t bg-accent"
              style={{ height: `${Math.max((bar.y / max) * 100, 2)}%` }}
            />
          </div>
          <span className="truncate font-mono text-2xs text-ink-3">{bar.x}</span>
        </div>
      ))}
    </figure>
  );
}

/** Barres horizontales — comparaison de grandeurs à libellés longs. */
export function Bars({
  rows,
  suffix = "",
}: {
  rows: { label: string; value: number; note?: string }[];
  suffix?: string;
}) {
  const max = Math.max(1, ...rows.map((row) => row.value));

  return (
    <ul className="space-y-2">
      {rows.map((row) => (
        <li
          key={row.label}
          title={`${row.label} — ${row.value}${suffix}`}
          className="flex items-center gap-3"
        >
          <span className="w-28 shrink-0 truncate text-xs text-ink-2">{row.label}</span>
          <span className="h-2 flex-1 overflow-hidden rounded-full bg-surface-3">
            <span
              className="block h-full rounded-full bg-accent"
              style={{ width: `${(row.value / max) * 100}%` }}
            />
          </span>
          <span className="w-24 shrink-0 text-right font-mono text-2xs text-ink-3" data-numeric>
            {row.value}
            {suffix}
            {row.note ? ` · ${row.note}` : ""}
          </span>
        </li>
      ))}
    </ul>
  );
}
