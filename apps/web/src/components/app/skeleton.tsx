/**
 * Ossatures d'attente.
 *
 * L'API met environ six cents millisecondes par appel, et une page en fait
 * parfois trois : sans ossature, on regarde un écran vide pendant deux
 * secondes en se demandant si le clic a été pris. La colonne latérale, elle,
 * reste en place — c'est tout l'intérêt d'un `loading.tsx` : seule la zone qui
 * charge est remplacée.
 *
 * Les blocs reprennent la géométrie réelle de chaque écran. Une ossature qui
 * ne ressemble pas à ce qui arrive produit un saut au moment du remplacement,
 * et ce saut est plus désagréable que l'attente qu'il masque.
 */

/** Bloc gris animé. L'animation s'efface si la personne l'a demandé. */
export function Bone({ className = "" }: { className?: string }) {
  return (
    <span
      aria-hidden
      className={`block animate-pulse rounded bg-surface-2 motion-reduce:animate-none ${className}`}
    />
  );
}

/** Entête d'écran : fil d'Ariane et titre. */
export function HeaderBones({ actions = 1 }: { actions?: number }) {
  return (
    <header className="flex h-14 items-center gap-3 border-b border-line px-6">
      <Bone className="h-3 w-24" />
      <div className="ml-auto flex gap-2">
        {Array.from({ length: actions }, (_, index) => (
          <Bone key={index} className="h-7 w-28" />
        ))}
      </div>
    </header>
  );
}

/** Tableau : une entête et des lignes. */
export function TableBones({ rows = 6, columns = 4 }: { rows?: number; columns?: number }) {
  return (
    <div className="px-6 py-6">
      <div className="overflow-hidden rounded-xl border border-line">
        <div className="flex gap-6 border-b border-line px-4 py-2.5">
          {Array.from({ length: columns }, (_, index) => (
            <Bone key={index} className="h-2.5 flex-1" />
          ))}
        </div>
        {Array.from({ length: rows }, (_, row) => (
          <div key={row} className="flex gap-6 border-b border-line/60 px-4 py-3 last:border-0">
            {Array.from({ length: columns }, (_, column) => (
              <Bone key={column} className={`h-3 flex-1 ${column === 0 ? "" : "opacity-60"}`} />
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}

/** Cartes en grille : catalogue, bibliothèque. */
export function CardsBones({ count = 4 }: { count?: number }) {
  return (
    <div className="grid gap-4 p-6 md:grid-cols-2 xl:grid-cols-3">
      {Array.from({ length: count }, (_, index) => (
        <div key={index} className="surface-card space-y-3 p-4">
          <Bone className="h-3.5 w-2/3" />
          <Bone className="h-2.5 w-1/3" />
          <Bone className="h-2.5 w-full opacity-60" />
          <Bone className="h-2.5 w-4/5 opacity-60" />
        </div>
      ))}
    </div>
  );
}

/** Colonnes du pipeline, avec leurs cartes. */
export function BoardBones() {
  return (
    <div className="grid gap-px overflow-x-auto bg-line md:grid-cols-2 xl:grid-cols-5">
      {Array.from({ length: 5 }, (_, column) => (
        <section key={column} className="min-h-[70vh] bg-surface-0 p-3">
          <div className="flex items-center gap-2 px-1 pb-3">
            <Bone className="size-1.5 rounded-full" />
            <Bone className="h-2.5 w-20" />
          </div>
          <div className="space-y-2">
            {Array.from({ length: column === 0 ? 3 : 1 }, (_, card) => (
              <div key={card} className="space-y-2 rounded-lg border border-line bg-surface-1 p-3">
                <Bone className="h-2 w-24" />
                <Bone className="h-3 w-full" />
                <div className="flex items-center justify-between pt-1">
                  <Bone className="h-2.5 w-12" />
                  <Bone className="h-3 w-20" />
                </div>
              </div>
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}

/** Fiche : un corps large et une colonne d'appoint. */
export function DetailBones() {
  return (
    <div className="px-6 py-6">
      <div className="flex flex-wrap items-start justify-between gap-6">
        <div className="space-y-2">
          <Bone className="h-6 w-64" />
          <Bone className="h-3 w-40" />
        </div>
        <div className="space-y-2">
          <Bone className="h-2.5 w-32" />
          <Bone className="h-4 w-44" />
        </div>
      </div>

      <div className="mt-8 grid gap-6 lg:grid-cols-[1fr_300px]">
        <div className="space-y-4">
          <Bone className="h-2.5 w-28" />
          {Array.from({ length: 4 }, (_, index) => (
            <div key={index} className="flex gap-3">
              <Bone className="mt-1 size-1.5 shrink-0 rounded-full" />
              <div className="w-full space-y-1.5">
                <Bone className="h-3 w-48" />
                <Bone className="h-2.5 w-64 opacity-60" />
              </div>
            </div>
          ))}
        </div>
        <div className="space-y-4">
          {Array.from({ length: 2 }, (_, index) => (
            <div key={index} className="surface-card space-y-2.5 p-4">
              <Bone className="h-2.5 w-20" />
              <Bone className="h-3 w-full" />
              <Bone className="h-3 w-3/4 opacity-60" />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

/** Tableau de bord : la rangée de tuiles puis les cadres de graphiques. */
export function DashboardBones() {
  return (
    <div className="px-6 py-6">
      <div className="grid grid-cols-2 gap-px overflow-hidden rounded-xl border border-line bg-line lg:grid-cols-5">
        {Array.from({ length: 5 }, (_, index) => (
          <div key={index} className="space-y-2 bg-surface-1 px-4 py-3.5">
            <Bone className="h-2.5 w-24" />
            <Bone className={index === 0 ? "h-7 w-24" : "h-5 w-10"} />
          </div>
        ))}
      </div>

      <div className="mt-6 grid gap-4 lg:grid-cols-2">
        {Array.from({ length: 4 }, (_, index) => (
          <div key={index} className="surface-card p-5">
            <Bone className="h-3 w-36" />
            <Bone className="mt-2 h-2.5 w-3/4 opacity-60" />
            <Bone className="mt-5 h-32 w-full opacity-40" />
          </div>
        ))}
      </div>
    </div>
  );
}
