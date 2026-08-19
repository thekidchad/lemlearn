# Visite guidée de la recette

L'API tourne sur AWS (`eu-west-3`, compte `986446886308`) ; le front n'est pas
déployé, il se lance en local et pointe dessus. Rien à configurer :
`apps/web/.env.local` est déjà renseigné.

```bash
pnpm dev          # http://localhost:3000
```

## Se connecter

```
marie@vulcain.fr
correcte-agrafe-cheval-pile
```

Compte propriétaire de l'organisme **Institut Vulcain**, promu `superadmin`
par `LEMLEARN_SUPERADMINS` — donc la vue « Organisations » est visible en plus
du reste. Retirer l'adresse de cette liste et redéployer lui retire l'accès à
sa connexion suivante.

## Le chemin qui montre le produit

1. **`/pipeline`** — le dossier `DOS-2026-629361` (Sécurité incendie — SSIAP 1)
   est celui qui porte de la matière. Sa pastille de complétude dit 3 pièces
   sur 13 : c'est vrai, et c'est le genre de chiffre qu'on préfère voir avant
   un contrôle qu'après.
2. **La fiche dossier** — journal horodaté à gauche, chaque événement chaîné au
   précédent. « Exporter le dossier » produit l'archive telle qu'un auditeur la
   recevrait, avec un manifeste qui **énumère ce qui manque et pourquoi**.
3. **`/sessions` → Session de février** — la feuille d'émargement, ses créneaux,
   sa contresignature. Les cases se changent directement.
4. **Espace apprenant** — `/apprenant?contactId=01KZXPH2QDQ7E9WXW2NGTXRPE8`
   (Léa Bertrand). Le module « Module vidéo réel » lit une vraie vidéo servie
   par CloudFront derrière une URL signée ; la piste sous le lecteur est la
   couverture *réelle*, pas la position. Le questionnaire est sous la vidéo,
   avec son corrigé après réponse.
5. **`/questionnaires`** — l'éditeur. Publier fige une version ; les passations
   restent attachées à celle qu'elles ont passée.
6. **`/abonnement`** puis **`/admin`** — la même consommation vue par le client
   et par l'équipe.

## Signer un document

Un lien de signature est prêt (usage unique, sept jours) :

```
http://localhost:3000/signer/Gk1tNPZ0jOOl4NIJ8zOwD9P6ZjpbrJP2F0XZQOsMUkA
```

Aucun courriel ne part en recette : le code à six chiffres se lit dans les
journaux de la fonction.

```bash
AWS_PROFILE=learnaly AWS_REGION=eu-west-3 aws logs filter-log-events \
  --log-group-name /aws/lambda/lemlearn-api-dev \
  --start-time $(( ($(date +%s) - 600) * 1000 )) \
  --query 'events[*].message' --output text | grep -o 'Votre code de signature : [0-9]*' | tail -1
```

À la fin, le PDF téléchargé porte une signature PAdES-B-T avec jeton
d'horodatage RFC 3161 :

```bash
openssl cms -verify -binary -inform DER -in signature.der \
  -content signed.bin -noverify        # « Verification successful »
```

Pour en émettre un autre : la fiche dossier, bouton « Envoyer à signer ».

## À savoir

Les contacts et dossiers `Nour Belkacem 18xxxx` sont les résidus de mes
parcours de test dans le navigateur — ils encombrent le pipeline sans rien
casser. Il n'existe pas encore de suppression : dans un produit dont l'argument
est la conservation de la preuve, elle demande d'être pensée (que supprime-t-on
d'un dossier qui porte un document scellé sous Object Lock ?) plutôt que
ajoutée en vitesse.
