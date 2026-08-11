# lemlearn

CRM, LMS vidéo et **chaîne de preuve horodatée** pour les organismes de
formation professionnelle français. Signature électronique interne (sans coût
par document), émargement numérique, exports Qualiopi prêts pour l'auditeur.

Le plan de construction complet est dans
[`docs/plan.md`](docs/plan.md) ; la charte graphique dans
[`docs/brand.md`](docs/brand.md).

## Organisation

```
apps/web/            Next.js 16 — landing, application, espace apprenant, super-admin
services/api/        Go 1.24 — API Lambda unique (chi), gabarits Typst, journal d'audit
infra/               AWS CDK v2 — DynamoDB, S3, Lambda, API Gateway
docs/                plan, charte, références de marque
```

## Prérequis

```bash
node 22 · pnpm 10 · go 1.24 · typst 0.14 (brew install typst)
```

Typst est nécessaire pour générer les documents. Sans lui, l'API démarre en
local et signale la génération comme indisponible.

## Démarrer

```bash
pnpm install

pnpm dev          # apps/web sur http://localhost:3000
pnpm api          # services/api sur http://localhost:8787
```

Vérifications rapides :

```bash
curl localhost:8787/health

# Convention de formation compilée avec un jeu de démonstration
open "http://localhost:8787/v1/documents/preview/convention"

# Zones de signature extraites du gabarit (page, x, y, largeur, hauteur en points)
curl "localhost:8787/v1/documents/preview/convention?zones=1" | jq
```

## Tests

```bash
pnpm api:test                       # tous les paquets Go
cd services/api && go test ./internal/platform/audit/ -v   # chaîne d'audit
```

Les tests de gabarit compilent réellement en PDF et vérifient que les zones de
signature sont déclarées, que deux rendus identiques produisent les mêmes
octets, et qu'un intitulé contenant `"`, `#` ou `\` ne casse pas la
compilation.

Les tests d'audit vérifient qu'une charge utile modifiée, un événement
supprimé, des événements réordonnés ou un événement forgé avec sa propre
empreinte valide sont tous détectés.

## Déploiement

```bash
cd services/api && make lambda && make layer   # binaire arm64 + layer Typst
cd ../../infra && pnpm synth                   # vérifie les deux piles
pnpm deploy -- --context env=dev
```

`make layer` télécharge le binaire Typst musl aarch64 et y joint les polices
Geist statiques ; il est monté en lecture seule sous `/opt` par la Lambda.

## Décisions structurantes

- **Une seule Lambda** pour toute l'API, routée par chi : un démarrage à froid,
  un artefact, un rollback. Une seconde fonction, dédiée, pour l'export de
  dossier (5 min, 2 Go de disque).
- **Typst plutôt qu'un navigateur sans tête** pour les PDF : compilation en
  dizaines de millisecondes, binaire statique, mise en page déterministe —
  approche reprise de khwiz, qui a fait cette migration en production.
- **Les zones de signature sont déclarées dans le gabarit**, pas dans le code
  d'envoi, et extraites par `typst query`. Le document sait où tombent ses
  cadres après mise en page ; personne d'autre ne peut le savoir.
- **Journal d'audit chaîné par hash**, dans une table séparée dont la politique
  IAM interdit `UpdateItem`, `DeleteItem` et `BatchWriteItem`. Le chaînage rend
  une altération détectable, la politique la rend impossible.
- **PDF reproductibles** : `SOURCE_DATE_EPOCH` est fixé à la date d'émission,
  donc deux compilations de la même convention donnent les mêmes octets — sans
  quoi l'empreinte SHA-256 du dossier de preuve ne prouverait rien.
- **Les données restent en France** (`eu-west-3`).
