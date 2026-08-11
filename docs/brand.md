# Charte graphique lemlearn

## Principe

lemlearn est consulté huit heures par jour par des gens dont le métier est de
tenir des dossiers. L'interface doit donc être **dense sans être bruyante** :
un seul accent, trois niveaux de texte, aucune ombre portée, un mouvement rare
et court. Tout ce qui attire l'œil doit le faire pour une raison — un statut de
preuve, une action, une alerte.

Les **documents PDF sont clairs**, à l'inverse de l'application. Ils sont
imprimés, annotés et relus par un auditeur ou un financeur.

## Marque

Le symbole fusionne un triangle de lecture et une coche qui s'en échappe : la
vidéo suivie, et la preuve qui en découle. Il est tracé en vectoriel dans
[logo.tsx](../apps/web/src/components/brand/logo.tsx) — deux variantes :

| Variante | Usage |
|---|---|
| `LogoMark` | glyphe seul, prend `currentColor` — en-têtes, boutons, favicons monochromes |
| `LogoSquare` | glyphe dans un carré arrondi sombre — icône d'application, favicon, en-tête de PDF |
| `Logo` | glyphe + wordmark `lemlearn` en bas de casse, `tracking -0.045em` |

Le wordmark s'écrit **toujours en bas de casse**, jamais « LemLearn » ni
« LEMLEARN ». Concept de référence généré au lancement du projet :
[logo-reference.png](brand/logo-reference.png).

## Couleurs

Palette de l'interface (jetons Tailwind v4 dans
[globals.css](../apps/web/src/app/globals.css)) :

| Jeton | Valeur | Rôle |
|---|---|---|
| `surface-0` | `#0B0C0F` | fond d'application |
| `surface-1` | `#121318` | cartes, panneaux |
| `surface-2` | `#1A1C23` | élévation, popovers, champs |
| `surface-3` | `#22242C` | rails, pistes de progression |
| `line` / `line-strong` | `#24262F` / `#32353F` | filets, survol et focus |
| `ink` / `ink-2` / `ink-3` | `#EDEEF2` / `#A0A4B0` / `#6B6F7D` | trois niveaux de texte, jamais quatre |
| `accent` / `accent-hover` | `#7C5CFF` / `#8F73FF` | action principale, sélection |
| `accent-ink` / `accent-dim` | `#C9BCFF` / `#1C1733` | texte et fond d'accent |
| `ok` / `warn` / `bad` | `#34D399` / `#FBBF24` / `#F87171` | **échelle de complétude d'un dossier** : complète, partielle, manquante |

Les couleurs sémantiques ne servent pas qu'aux notifications : ce sont les trois
états d'une pièce de preuve, et elles doivent garder ce sens partout.

Une palette a été générée en parallèle
([palette-reference.png](brand/palette-reference.png)) ; ses couleurs
sémantiques (`#2E8B57`, `#D9A06F`, `#B22222`) ont été écartées, trop sombres et
trop saturées pour tenir le contraste AA sur un fond `#0B0C0F`. Les valeurs
ci-dessus ont été retenues à leur place.

Palette des documents PDF (voir [chrome.go](../services/api/internal/platform/doc/chrome.go)) :
`ink #10131A`, `muted #5B6170`, `faint #8A90A0`, `hairline #DFE2E8`,
`accent #4B37B8` — le violet est assombri pour rester lisible à l'impression.

## Typographie

**Geist** partout — à l'écran via `next/font`, dans les PDF via les TTF
statiques embarqués dans le binaire Go
([fonts.go](../services/api/internal/platform/doc/fonts.go)). Ce sont les mêmes
fichiers : l'application et le document imprimé partagent exactement la même
fonte.

- Échelle : `2xs` 11px · `xs` 12px · `sm` 13px (densité par défaut de l'app) ·
  `base` 15px · puis 20 / 28 / 40 pour les titres.
- Titres : `letter-spacing -0.025em` à `-0.045em`, `text-wrap: balance`.
- **Tous les chiffres en `tabular-nums`** : montants, durées, scores, dates,
  références. Un tableau dont les chiffres dansent est illisible.
- `Geist Mono` pour tout ce qui est identifiant, horodatage ou empreinte —
  c'est le signal visuel de « donnée vérifiable ».

## Mouvement

120 ms, `cubic-bezier(.2,.8,.2,1)`, jamais plus de 200 ms. Aucune animation
d'entrée au défilement. `prefers-reduced-motion` coupe tout.

## Photographie

Photographie documentaire, lumière naturelle, palette désaturée, profondeur de
champ courte. Des gens qui travaillent, pas qui posent : aucun sourire face
caméra, aucun décor de studio, aucune poignée de main.

Les trois images de la page d'accueil
([photos/](../apps/web/public/photos)) sont **générées** et ne représentent
donc aucune personne réelle. Elles servent d'illustration d'ambiance
uniquement : elles ne doivent jamais être présentées comme des clients, ni
accompagner un témoignage. Un témoignage exige une vraie personne, un vrai
accord écrit et une vraie photo.

## Règles à ne pas enfreindre

1. Un seul accent. Si un deuxième violet apparaît, c'est un bug.
2. Pas d'ombre portée sur fond sombre — l'élévation se fait par la surface et
   le filet lumineux supérieur (`.surface-card`).
3. Trois niveaux de texte au maximum. Un quatrième gris signifie que la
   hiérarchie est mal posée.
4. Le vert, l'ambre et le rouge sont réservés à l'état des preuves.
5. Aucun texte sous 11px, aucun contraste sous AA.
