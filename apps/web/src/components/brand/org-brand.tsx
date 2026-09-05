import type { Brand } from "@/lib/api";

/**
 * L'enseigne d'un organisme de formation.
 *
 * C'est la pièce centrale de la marque blanche : partout où l'ancien logo
 * lemlearn s'affichait — coque de l'organisme, espace apprenant, page de
 * signature — c'est désormais celle-ci. Un stagiaire ne doit jamais voir le
 * nom de l'outil : il n'a pas de relation avec nous, il en a une avec son
 * organisme.
 *
 * Sans logo déposé, on rend un monogramme dans la couleur de l'organisme
 * plutôt qu'un logo emprunté. Deux lettres justes valent mieux qu'une marque
 * qui n'est pas la sienne, et l'organisme peut ouvrir son compte sans avoir
 * de fichier sous la main.
 *
 * Avec un logo, le nom n'est pas répété à côté : un logo porte presque
 * toujours le nom de la maison — c'est ce qui en fait un logo — et l'écrire
 * une seconde fois l'affichait en double, puis débordait de la colonne.
 */
export function OrgBrand({ brand, className }: { brand: Brand; className?: string }) {
  if (brand.logoUrl) {
    return (
      <span className={`inline-flex min-w-0 items-center ${className ?? ""}`}>
        <OrgMark brand={brand} />
      </span>
    );
  }

  return (
    <span className={`inline-flex min-w-0 items-center gap-2.5 ${className ?? ""}`}>
      <OrgMark brand={brand} />
      {/* min-w-0 sur l'élément lui-même : sans lui, un élément de flex ne
          descend pas sous sa largeur de contenu, et « truncate » ne tronque
          rien — le nom sort de la colonne au lieu de se couper. */}
      <span className="min-w-0 truncate text-[0.9375rem] font-semibold tracking-[-0.045em] text-ink">
        {brand.name}
      </span>
    </span>
  );
}

/**
 * La marque seule, sans le nom : pour les endroits contraints — un favicon,
 * une pastille, un en-tête de courrier.
 *
 * Le logo est contenu et non recadré : un organisme dépose ce qu'il a, souvent
 * un rectangle large, et le rogner en carré couperait la moitié du nom.
 */
export function OrgMark({ brand, size = 28 }: { brand: Brand; size?: number }) {
  if (brand.logoUrl) {
    return (
      // Une balise <img> et non next/image : la source est un compartiment S3
      // dont l'hôte change d'un environnement à l'autre, et l'optimiseur
      // refuse un domaine qu'il ne connaît pas — un logo qui ne s'affiche
      // pas serait pire que non optimisé.
      // eslint-disable-next-line @next/next/no-img-element
      <img
        src={brand.logoUrl}
        alt={brand.name}
        style={{ height: size, maxWidth: size * 5 }}
        className="w-auto shrink-0 object-contain"
      />
    );
  }

  return (
    <span
      aria-hidden
      style={{ width: size, height: size, background: brand.accent, color: brand.accentInk }}
      className="flex shrink-0 items-center justify-center rounded-md text-2xs font-semibold tracking-[-0.02em]"
    >
      {brand.monogram}
    </span>
  );
}

/**
 * Les variables de thème de l'organisme, à poser sur un conteneur.
 *
 * Elles écrasent l'accent du produit sans toucher au reste de la palette :
 * laisser un organisme choisir ses fonds et ses gris produirait surtout des
 * écrans illisibles, alors qu'une seule teinte suffit à ce qu'il se
 * reconnaisse.
 */
export function brandStyle(brand: Brand): React.CSSProperties {
  return {
    "--color-accent": brand.accent,
    "--color-accent-ink": brand.accentInk,
  } as React.CSSProperties;
}
