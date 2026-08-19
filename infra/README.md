# Déploiement

Deux piles : `Lemlearn-Data-<env>` (tables et compartiments) et
`Lemlearn-Compute-<env>` (Lambda et passerelle HTTP). Elles sont séparées parce
que le calcul se redéploie plusieurs fois par jour et les données jamais.

## Région

**eu-west-3 (Paris).** Ce n'est pas un détail de configuration : « données
hébergées en France » figure sur la page d'accueil et dans la convention type.
Le profil AWS peut être configuré sur une autre région, le déploiement force
celle-ci.

## Profil et droits

Le profil `learnaly` doit porter la politique
[`deploy-policy.json`](deploy-policy.json), portée par région et par préfixe de
ressource — `lemlearn-*`, `cdk-lemlearn26-*`.

Ce que CDK exige en plus d'un déploiement SAM classique :

| Droit | Pourquoi |
|---|---|
| `ssm:PutParameter` sur `/cdk-bootstrap/lemlearn26/*` | CDK y écrit sa version d'amorçage et la relit à chaque déploiement. Sans ce paramètre, aucun déploiement ne démarre. |
| `ecr:CreateRepository` sur `cdk-lemlearn26-*` | Le gabarit d'amorçage crée un dépôt de conteneurs, même si aucune image n'est déployée ici. |
| `sts:AssumeRole` sur `cdk-lemlearn26-*` | CDK endosse ses propres rôles pour publier les artefacts et exécuter CloudFormation. |
| `kms:CreateKey` | La clé qui chiffre les pièces d'identité des apprenants. |

**Qualificatif `lemlearn26`.** Un stack `CDKToolkit` par défaut existe déjà sur
certains comptes, parfois bloqué en `DELETE_FAILED` avec un paramètre SSM que
personne n'a le droit de supprimer. Le qualificatif crée un jeu de ressources
d'amorçage parallèle plutôt que d'aller réparer un état qui n'est pas le nôtre.
Il est fixé dans [`cdk.json`](cdk.json) et doit être repris à l'amorçage.

## Déployer

```bash
# 1. Artefacts : binaire ARM64 (20 Mo) et layer Typst + polices (43 Mo)
cd services/api && make lambda && make layer

# 2. Amorçage, une seule fois par compte et par région
cd ../../infra
AWS_PROFILE=learnaly AWS_REGION=eu-west-3 \
  npx cdk bootstrap aws://<compte>/eu-west-3 \
  --qualifier lemlearn26 --toolkit-stack-name CDKToolkit-lemlearn

# 3. Déploiement
AWS_PROFILE=learnaly AWS_REGION=eu-west-3 \
  npx cdk deploy --all --context env=dev --require-approval never
```

L'URL de l'API sort en `ApiUrl` à la fin du déploiement.

## Chaîne vidéo

Le dépôt et le transcodage se provisionnent seuls. La diffusion demande une
paire de clés CloudFront, qui n'a rien à faire dans un dépôt :

```bash
openssl genrsa -out cloudfront.pem 2048
openssl rsa -pubout -in cloudfront.pem -out cloudfront.pub

LEMLEARN_CDN_PUBLIC_KEY="$(cat cloudfront.pub)" \
AWS_PROFILE=learnaly AWS_REGION=eu-west-3 \
  npx cdk deploy Lemlearn-Compute-dev --context env=dev
```

La clé privée correspondante se pose ensuite en variable `LEMLEARN_CDN_KEY` sur
la fonction. Sans les trois valeurs, l'hébergement vidéo reste actif pour le
dépôt et le transcodage, et les routes de lecture répondent 409 avec un motif —
un organisme qui ne fait que du présentiel n'a pas de vidéo à diffuser, et le
reste du produit ne doit pas en dépendre.

Le point d'entrée MediaConvert n'est pas à renseigner : la Lambda le demande au
service au démarrage, et retombe sur le point d'entrée régional si l'appel
échoue.

## Satisfaction à froid

La clôture d'une session programme la relance à trois mois pour chaque
inscrit ; une règle EventBridge (`lemlearn-satisfaction-froid-<env>`) invoque
la fonction API à 7 h UTC avec `{"task":"satisfaction-froid"}`. La même
fonction sert l'API et ce travail — elle distingue les deux sur la forme de
l'événement.

Le traitement relit les échéances du mois courant **et du mois précédent** :
une panne décale la relance, elle ne la perd pas.

Déclencher manuellement :

```bash
AWS_PROFILE=learnaly AWS_REGION=eu-west-3 aws lambda invoke \
  --function-name lemlearn-api-dev \
  --payload "$(printf '{"task":"satisfaction-froid"}' | base64)" /dev/stdout
```

Ce déclenchement manuel — et l'inspection de la règle — demandent
`lambda:InvokeFunction` et `events:Describe/ListRules` dans la politique du
profil. Le déploiement, lui, n'en a pas besoin : CloudFormation crée la règle
avec le rôle d'exécution CDK.

## Ce qui survit à un `cdk destroy`

Délibérément, et c'est à savoir avant de lancer quoi que ce soit :

- **la table d'audit** (`RETAIN` en toutes circonstances) — perdre un journal
  de preuve pour libérer un environnement de test est une mauvaise habitude ;
- **le compartiment des documents** (`RETAIN`, Object Lock activé) — en
  production la rétention est de dix ans en mode `COMPLIANCE`, ce que ni un
  administrateur ni le compte racine ne peut contourner.

En `dev`, l'Object Lock est en mode `GOVERNANCE` sur un jour : de quoi exercer
le mécanisme sans immobiliser un compartiment pendant une décennie.

## Variables attendues par le service

Renseignées par la pile de calcul, sauf les deux dernières :

| Variable | Rôle |
|---|---|
| `LEMLEARN_APP_URL` | adresse publique du front. Elle compose les liens de signature et de satisfaction envoyés par courriel, et l'origine autorisée en CORS. À poser au déploiement : `LEMLEARN_APP_URL=https://…` devant `cdk deploy`. |
| `LEMLEARN_SUPERADMINS` | adresses de l'équipe lemlearn, séparées par des virgules. Le rôle s'attribue *et se retire* d'après cette liste à la connexion. |
| `LEMLEARN_TABLE`, `LEMLEARN_AUDIT_TABLE` | tables DynamoDB |
| `LEMLEARN_DOCUMENTS_BUCKET`, `LEMLEARN_IDENTITY_BUCKET`, `LEMLEARN_VIDEO_BUCKET` | compartiments |
| `TYPST_PATH`, `TYPST_FONT_PATH` | binaire et polices montés par le layer |
| `RESEND_API_KEY` | **exigée en production seulement.** Hors production, les courriels sont journalisés — le domaine d'envoi n'y est de toute façon pas vérifié. |
| `LEMLEARN_SEAL_CERT`, `LEMLEARN_SEAL_KEY` | **exigées en production seulement.** À défaut, un certificat auto-signé est généré, dont le nom porte « sans valeur » pour qu'un document de recette ne puisse pas passer pour un document contractuel. |
