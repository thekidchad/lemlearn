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

# Base locale : DynamoDB Local dans Docker, puis création des tables
pnpm db:up
pnpm db:setup

pnpm dev          # apps/web sur http://localhost:3000
pnpm api          # services/api sur http://localhost:8787
```

L'API démarre sans base et sans Typst : elle signale alors les routes
concernées comme indisponibles plutôt que de refuser de se lancer.

Vérifications rapides :

```bash
curl localhost:8787/health

# Convention de formation compilée avec un jeu de démonstration
open "http://localhost:8787/v1/documents/preview/convention"

# Zones de signature extraites du gabarit (page, x, y, largeur, hauteur en points)
curl "localhost:8787/v1/documents/preview/convention?zones=1" | jq

# Parcours complet
curl -X POST localhost:8787/v1/auth/register -H 'Content-Type: application/json' \
  -d '{"orgName":"Institut Vulcain","email":"marie@vulcain.fr",
       "password":"correcte-agrafe-cheval-pile","firstName":"Marie","lastName":"Dubreuil"}'

curl -c /tmp/lem.txt -X POST localhost:8787/v1/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"marie@vulcain.fr","password":"correcte-agrafe-cheval-pile"}'

curl -b /tmp/lem.txt localhost:8787/v1/me
curl -b /tmp/lem.txt localhost:8787/v1/files            # pipeline
curl -b /tmp/lem.txt localhost:8787/v1/files/<id>/timeline   # journal vérifié

# Parcours de signature. En local, la réponse porte un `devLink` : aucun
# courriel ne part réellement, le code se lit dans les journaux de l'API.
curl -b /tmp/lem.txt -X POST localhost:8787/v1/files/<id>/signatures \
  -H 'Content-Type: application/json' \
  -d '{"kind":"convention","reference":"CONV-2026-0143","role":"client",
       "signerName":"Léa Bertrand","signerEmail":"lea@example.fr"}'

curl localhost:8787/v1/sign/<token>              # ce que voit le signataire
curl localhost:8787/v1/sign/<token>/document     # le PDF à signer
curl -X POST localhost:8787/v1/sign/<token>/otp  # envoi du code
curl -X POST localhost:8787/v1/sign/<token>/confirm -d @signature.json
curl localhost:8787/v1/sign/<token>/sealed       # le document signé
```

## Tests

```bash
pnpm db:up        # DynamoDB Local, requis par les tests d'intégration
pnpm api:test     # tous les paquets Go
```

Les tests d'intégration tournent contre **le vrai moteur DynamoDB**, jamais
contre une imitation en mémoire : conditions d'écriture, transactions
multi-tables et cohérence forte sont précisément ce sur quoi repose
l'intégrité du journal, et un faux client ne les reproduit pas. Sans
`DDB_ENDPOINT`, ils sont ignorés plutôt que d'échouer.

Les tests de gabarit compilent réellement en PDF et vérifient que les zones de
signature sont déclarées, que deux rendus identiques produisent les mêmes
octets, et qu'un intitulé contenant `"`, `#` ou `\` ne casse pas la
compilation.

Les tests d'audit vérifient qu'une charge utile modifiée, un événement
supprimé, des événements réordonnés ou un événement forgé avec sa propre
empreinte valide sont tous détectés — et que la chaîne reste vérifiable après
un aller-retour complet par DynamoDB.

Les tests de signature exercent le parcours complet contre le vrai moteur :
émission, ouverture, code, apposition, scellement — puis vérifient que le
journal contient exactement les sept événements attendus dans l'ordre, que le
code n'apparaît nulle part, que trois essais épuisent la tentative, qu'un
tracé de deux dixièmes de seconde est refusé, et qu'un octet modifié dans le
document archivé fait échouer la vérification d'intégrité.

Les tests d'assiduité vérifient qu'un apprenant qui rejoue trois fois la même
minute reste à une minute de couverture, qu'une progression impossible (cinq
minutes jouées en dix secondes réelles) est refusée, que la lecture en
arrière-plan ne compte pas, et qu'un saut ne colorie pas l'intervalle sauté.

Les tests de questionnaire vérifient que cocher toutes les options ne rapporte
rien, qu'une passation reste attachée à sa version même après publication
d'une nouvelle, que le corrigé ne part jamais vers l'apprenant avant sa
soumission, et que le mélange des questions est reproductible pour un auditeur.

Les tests du CRM couvrent l'isolation entre organisations, l'unicité d'une
adresse e-mail entre organisations, et le fait que quatre déplacements
concurrents du même dossier laissent une chaîne d'audit intègre.

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
- **Sessions opaques et révocables** plutôt que JWT : révoquer un JWT impose de
  toute façon une liste de révocation consultée à chaque requête, donc un
  aller-retour en base. Autant stocker la session et pouvoir réellement la
  couper — indispensable pour l'impersonation.
- **L'isolation entre clients est structurelle**, pas conditionnelle : toutes
  les données d'une organisation partagent la clé de partition `ORG#<id>`, et
  celle-ci vient de la session, jamais de l'URL. Un bug de filtre ne peut pas
  exposer les données d'un autre organisme, parce que la requête n'atteint
  jamais leur partition.
- **Le scellement est écrit sur la bibliothèque standard.** CMS et PAdES sont
  spécifiés et stables depuis vingt ans ; les écrire ici évite une dépendance
  de plus dans une Lambda et rend visible ce qui compose exactement la
  signature apposée à un document contractuel. La contrepartie est assumée :
  le résultat est validé par OpenSSL, pas par nous-mêmes.
- **La signature est rendue *dans* sa zone**, pas tamponnée après coup sur un
  PDF existant : le document signé est un rendu de plein droit, reproductible
  à l'octet près, et il n'y a aucun post-traitement dont il faudrait démontrer
  la fidélité au document présenté au signataire.
- **Le jeton de signature n'est jamais rendu à l'organisme.** Un administrateur
  qui pourrait le récupérer pourrait signer à la place du bénéficiaire, et le
  dossier de preuve ne le montrerait pas. Seule exception, bornée au
  développement local où aucun courriel ne part.
- **Les données restent en France** (`eu-west-3`).

## Scellement PAdES

Le document signé porte une **signature PAdES-B-T incorporée** : signature
CAdES détachée (RFC 5652) apposée par mise à jour incrémentale du PDF, avec un
jeton d'horodatage RFC 3161 obtenu auprès d'une autorité tierce.

Tout est écrit sur la bibliothèque standard de Go — `encoding/asn1`,
`crypto/x509` — dans trois paquets lisibles :

| Paquet | Rôle |
|---|---|
| [`platform/cms`](services/api/internal/platform/cms) | SignedData CMS détaché, attributs CAdES-BES (contentType, messageDigest, signingTime, signing-certificate-v2) |
| [`platform/pdfsig`](services/api/internal/platform/pdfsig) | révision incrémentale : dictionnaire `/Sig`, champ AcroForm, `/ByteRange`, xref chaînée par `/Prev` |
| [`platform/tsa`](services/api/internal/platform/tsa) | client RFC 3161, avec nonce contre le rejeu |

Les octets de la révision d'origine ne sont **jamais réécrits** : un
vérificateur peut prouver que le document présenté au signataire est
exactement celui que contient le fichier signé. `/ByteRange` couvre tout le
fichier sauf la chaîne hexadécimale de la signature elle-même.

Les tests ne se contentent pas de vérifier notre propre travail : ils
extraient la signature du PDF produit et la font valider par **`openssl cms
-verify`**, puis contrôlent qu'un octet modifié dans le corps du document la
fait échouer.

**Certificat.** En production, `LEMLEARN_SEAL_CERT` et `LEMLEARN_SEAL_KEY`
portent un cachet d'organisation (≈300 €/an chez Certigna ou ChamberSign),
injectés par Secrets Manager ; sans eux, le service refuse de démarrer hors
local. En développement, un certificat auto-signé est généré, dont le nom
porte explicitement la mention « certificat de développement — sans valeur »
pour qu'un document de test ne puisse pas passer pour un document contractuel.

**Horodatage.** FreeTSA par défaut ; une autorité qualifiée eIDAS est
recommandée en production. Si l'autorité est injoignable, le document est
scellé sans jeton — signature valide, date non opposable — plutôt que de
refuser une signature déjà consentie, et le dossier de preuve laisse le champ
**vide** au lieu d'afficher une heure serveur.
