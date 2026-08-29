/**
 * Visuel d'une formation.
 *
 * Sans image déposée, on rend une bande dans la couleur de l'organisme portant
 * les initiales de la formation — pas un rectangle gris. Un espace apprenant
 * est souvent ouvert par quelqu'un qui suit deux formations à la fois : c'est
 * le visuel qui les distingue avant le titre, et une case vide rendrait la
 * liste illisible au moment précis où elle doit se lire d'un coup d'œil.
 */
export function CourseCover({
  title,
  url,
  ratio = "21 / 9",
}: {
  title: string;
  url?: string;
  ratio?: string;
}) {
  if (url) {
    return (
      // Une balise <img> et non next/image : la source est un compartiment S3
      // dont l'hôte change d'un environnement à l'autre, et l'optimiseur
      // refuse un domaine qu'il ne connaît pas.
      // eslint-disable-next-line @next/next/no-img-element
      <img
        src={url}
        alt=""
        style={{ aspectRatio: ratio }}
        className="w-full rounded-lg object-cover"
      />
    );
  }

  return (
    <span
      aria-hidden
      style={{ aspectRatio: ratio }}
      className="flex w-full items-end justify-start overflow-hidden rounded-lg bg-accent/12 p-4"
    >
      <span className="text-3xl font-semibold tracking-[-0.05em] text-accent opacity-70">
        {initials(title)}
      </span>
    </span>
  );
}

/**
 * Deux lettres tirées du titre, en ignorant les mots de liaison — même règle
 * que le monogramme d'un organisme, pour que le produit ne parle qu'une langue.
 */
function initials(title: string): string {
  const liaison = new Set(["de", "du", "des", "la", "le", "les", "et", "d", "l", "en", "aux", "à"]);
  const mots = title
    .split(/[^\p{L}\p{N}]+/u)
    .filter(Boolean)
    .filter((mot) => !liaison.has(mot.toLowerCase()));
  if (mots.length === 0) return "•";
  if (mots.length === 1) return [...mots[0]].slice(0, 2).join("").toUpperCase();
  return (mots[0][0] + mots[1][0]).toUpperCase();
}
