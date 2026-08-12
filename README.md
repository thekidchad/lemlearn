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

L'application est à `/connexion`. Les pages sont des composants serveur qui
appellent l'API en relayant le cookie de session : le jeton ne transite jamais
par le JavaScript du navigateur, donc une faille XSS ne donne pas accès aux
dossiers.

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

# Catalogue et espace apprenant
curl -b /tmp/lem.txt -X POST localhost:8787/v1/courses -d '{...}'
curl -b /tmp/lem.txt -X POST localhost:8787/v1/courses/<id>/modules -d '{...}'
curl -b /tmp/lem.txt -X POST localhost:8787/v1/quizzes -d '{...}'
curl -b /tmp/lem.txt -X POST localhost:8787/v1/quizzes/<id>/versions/1/publish
curl -b /tmp/lem.txt -X POST localhost:8787/v1/sessions/<id>/enrollments -d '{"contactId":"…"}'

# Le lecteur vidéo émet un signal toutes les cinq secondes
curl -b /tmp/lem.txt -X POST \
  "localhost:8787/v1/learn/<session>/courses/<course>/modules/<module>/beat" \
  -d '{"fromMs":0,"toMs":5000,"rate":1,"focused":true}'

curl -b /tmp/lem.txt "localhost:8787/v1/learn"   # tableau de bord apprenant

# Le dossier probatoire complet, tel qu'il est remis à un auditeur
curl -b /tmp/lem.txt -X POST -o dossier.zip localhost:8787/v1/files/<id>/export
```

L'archive contient les documents scellés relus depuis l'archivage, les relevés
de connexion et d'évaluation générés à la volée, l'attestation si elle est
délivrable, le journal d'audit en CSV et un manifeste d'empreintes SHA-256 :

```
  générée   dossier.json                          2 178 o
  archivée  documents/CONV-2026-0900.pdf         87 428 o
  générée   releves/releve-de-connexion.pdf      39 296 o
  générée   releves/releve-evaluation.pdf        41 339 o
  générée   journal-audit.csv                     3 299 o
  générée   manifeste.json
```

Le manifeste distingue les pièces **archivées** — qui portent leur propre
signature PAdES — des pièces **générées**, qui n'engagent que la fidélité de
nos données. Et il énumère ce qui manque, avec le motif : un dossier incomplet
dont on connaît les trous vaut mieux qu'un dossier qui prétend être complet.

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

Les tests d'émargement vérifient qu'une session en présentiel se découpe en
demi-journées et une session asynchrone en modules, qu'un créneau déjà émargé
ne se réécrit pas, qu'une absence sans motif est refusée, et qu'une absence
n'entre pas dans les heures facturables.

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
- **L'émargement porte toujours sur un créneau**, jamais sur une formation ni
  sur un module : c'est la présence à un moment donné qui est attestée, et
  c'est ce qu'un contrôleur recompte. Les créneaux sont dérivés de la session
  plutôt que saisis — une feuille dont les créneaux ne correspondent pas à ce
  qui a été conventionné ne prouve rien.
- **Les données restent en France** (`eu-west-3`).

## Vidéo

Le chemin est AWS de bout en bout : le navigateur dépose le fichier
directement dans S3 par URL présignée, MediaConvert le transcode en HLS
multi-débit (360p / 540p / 720p), CloudFront le diffuse derrière des URL
signées valables quinze minutes. L'API ne voit jamais passer un octet de
vidéo — la faire transiter par une Lambda imposerait de la dimensionner pour
des fichiers dont elle n'a rien à faire.

La signature CloudFront est écrite sur la bibliothèque standard
([video.go](services/api/internal/platform/../video/video.go)) : une politique
JSON, une signature RSA-SHA1, trois paramètres d'URL. Deux pièges y sont
documentés parce qu'ils coûtent des heures — l'alphabet base64 particulier de
CloudFront (`+=/` deviennent `-_~`), et l'ordre imposé des champs de la
politique, qui interdit d'utiliser une map Go.

La protection ne vise pas l'impossible : un apprenant déterminé peut filmer
son écran. Elle vise le partage d'URL, qui est le risque réel.

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
