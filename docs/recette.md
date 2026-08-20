# Recette — les chemins de bout en bout

Ce qu'il faut ouvrir, dans quel ordre, et **ce qui doit se passer**. Chaque
chemin se tient debout tout seul ; le premier et le deuxième se complètent.

## Avant de commencer

```bash
pnpm dev        # http://localhost:3000 — l'API est déjà déployée sur AWS
```

Deux comptes, pour deux points de vue :

| Compte | Mot de passe | À quoi il sert |
|---|---|---|
| `marie@vulcain.fr` | `correcte-agrafe-cheval-pile` | Organisme **Institut Vulcain**, avec de la matière : une convention scellée, une vidéo, des questionnaires. Aussi `superadmin`, donc la vue équipe. |
| celui que vous créerez | le vôtre | Un organisme **vide**, pour éprouver le parcours d'entrée et les états vides. |
| `lea@example.fr` | `barque-tilleul-orage-72` | Une **apprenante** de l'Institut Vulcain : elle ne voit que son parcours. |

Aucun courriel ne part en recette : ils sont journalisés. Pour lire un code de
signature :

```bash
AWS_PROFILE=learnaly AWS_REGION=eu-west-3 aws logs filter-log-events \
  --log-group-name /aws/lambda/lemlearn-api-dev \
  --start-time $(( ($(date +%s) - 600) * 1000 )) \
  --query 'events[*].message' --output text \
  | grep -o 'Votre code de signature : [0-9]*' | tail -1
```

---

## 1. Le parcours commercial — du prospect au dossier scellé

*Compte : le vôtre, créé à l'étape 1. C'est le chemin qu'un organisme fait le
premier jour.*

1. **`/` → « Commencer l'essai »**. Créez votre organisme.
   → Vous arrivez connecté sur le pipeline, vide. Votre organisme s'affiche en
   bas de la barre latérale.
2. **`/contacts` → « Nouveau »**. Un apprenant, avec courriel, date **et lieu**
   de naissance.
   → Il apparaît dans la liste. Cliquez son nom : la fiche s'ouvre.
3. **`/pipeline` → « Nouveau dossier »**, en le rattachant à cet apprenant.
   → La carte affiche **0 %** de complétude. C'est normal et c'est le sujet :
   ce chiffre va monter.
4. **Ouvrez le dossier → « Envoyer à signer »**. Le nom et le courriel sont
   pré-remplis depuis la fiche.
   → Le journal, à gauche, gagne « Document généré » puis « Document envoyé à
   signer », horodatés à la seconde.
5. **Récupérez le lien** dans les journaux (même commande que ci-dessus, en
   cherchant `signer/`), ouvrez-le **dans une fenêtre privée** — c'est ce que
   voit le signataire, qui n'a pas de compte.
   → La convention s'affiche entière, avec son empreinte SHA-256 sous le
   document.
6. **Cochez, demandez le code, saisissez-le**, recopiez la mention, signez dans
   le cadre.
   → Le bouton reste inerte tant que la mention n'est pas exacte et que rien
   n'est tracé. Puis : récépissé avec l'empreinte du document **scellé** et la
   mention « jeton d'un tiers horodateur (RFC 3161) ».
7. **Revenez au dossier.**
   → « Document signé » et « Document scellé » sont au journal, et la
   complétude a bougé.
8. **« Exporter le dossier ».**
   → Une archive se télécharge, et le bouton annonce le décompte : *n pièces ·
   m manquantes*. Ouvrez `manifeste.json` : chaque pièce absente est nommée
   **avec son motif**.

**Ce qu'il faut essayer de casser :** saisir trois fois un mauvais code (le lien
se bloque), signer d'un point unique (« tracé trop pauvre »), rouvrir le lien
après signature (usage unique).

---

## 2. Le parcours pédagogique — de la formation à l'attestation

*Compte : `marie@vulcain.fr`, où la vidéo et les questionnaires existent déjà.*

1. **`/questionnaires` → « Nouveau »**. Un contrôle *après module* avec deux
   questions, un corrigé, une explication. Enregistrez, puis **publiez**.
   → Après publication, « Enregistrer » devient « Enregistrer comme nouvelle
   version » : une version publiée ne se modifie plus.
2. **`/catalogue` → « Nouvelle formation »**. Remplissez les mentions, cochez
   « Publier ».
   → Sans objectifs pédagogiques ni public visé, la publication est **refusée**
   avec le motif exact. C'est voulu : ces mentions partent sur la convention.
3. **Ouvrez la formation → « Ajouter un module »**, en lui attachant le
   questionnaire publié.
4. **« Téléverser une vidéo »** sur ce module (n'importe quel MP4 court).
   → Le bouton passe par *réservation*, *téléversement*, *transcodage*. Une
   minute plus tard, rechargez : le module annonce « vidéo attachée ».
5. **`/sessions` → « Nouvelle session »** sur cette formation, puis
   **« Inscrire »** votre apprenant.
   → Il apparaît en ligne de la feuille d'émargement.
6. **`/apprenant`** → choisissez cet apprenant.
   → Son parcours s'affiche comme il le voit. Ouvrez le module : la vidéo se
   lit réellement.
7. **Regardez une vingtaine de secondes, puis revenez en arrière et rejouez le
   même passage.**
   → La piste sous le lecteur — la **couverture réelle** — n'avance pas deux
   fois pour le même passage. C'est là toute la différence avec un compteur de
   temps passé.
8. **Répondez au questionnaire sous la vidéo**, volontairement à moitié faux.
   → Score, corrigé question par question, explication, et le compte des
   tentatives restantes.
9. **Retour sur `/sessions/{id}`** : cochez les présences, puis
   **« Contresigner la feuille »**.
   → L'en-tête passe de l'orange « à contresigner » au vert « Contresignée
   par… ».
10. **« Clôturer ».**
    → « Relances programmées. » La satisfaction à froid part trois mois après
    la fin de session, pour chaque inscrit. Recliquer : « cette session est
    déjà clôturée ».

---

## 3. Le parcours de l'apprenant — son compte, pas le vôtre

*Un apprenant ne s'inscrit pas lui-même : un espace en libre inscription
laisserait n'importe qui se déclarer stagiaire de l'organisme. C'est
l'organisme qui lui ouvre l'accès.*

1. **Fiche d'un apprenant → « Ouvrir l'accès »** (il lui faut une adresse).
   → Un courriel part avec un lien personnel valable quatorze jours. En
   recette, récupérez-le dans les journaux :

   ```bash
   AWS_PROFILE=learnaly AWS_REGION=eu-west-3 aws logs filter-log-events      --log-group-name /aws/lambda/lemlearn-api-dev      --start-time $(( ($(date +%s) - 600) * 1000 ))      --query 'events[*].message' --output text      | grep -oE 'invitation/[A-Za-z0-9_-]+' | tail -1
   ```

2. **Ouvrez `/invitation/{jeton}` dans une fenêtre privée**, choisissez un mot
   de passe.
   → Vous entrez directement dans l'espace : demander de se reconnecter juste
   après avoir choisi un mot de passe est une étape que personne ne comprend.
3. **Regardez la barre latérale.**
   → Un seul lien : « Mon parcours ». Les écrans de l'organisme ne lui sont pas
   seulement cachés, ils lui sont fermés — tapez `/pipeline` dans la barre
   d'adresse pour le vérifier : « Cet écran n'est pas pour vous ».
4. **Ouvrez un module et regardez la vidéo.**
   → Le temps est enregistré sous son propre compte, plus sous un `contactId`
   passé dans l'URL par un administrateur.
5. **Réinvitez la même personne depuis sa fiche.**
   → Le lien précédent cesse de valoir, et aucun second compte n'est créé.

Ou, tout simplement : `lea@example.fr` / `barque-tilleul-orage-72`, déjà
ouverte sur l'Institut Vulcain.

## 4. Le parcours conformité et RGPD

*Compte : `marie@vulcain.fr`.*

1. **`/qualiopi`.**
   → Taux de complétude, **pièces qui manquent le plus souvent** (une pièce
   absente partout est un défaut de procédure, pas un oubli), et les dossiers
   les plus incomplets en tête.
2. **Fiche d'un apprenant → « Déposer une pièce »** (JPEG, PNG, HEIC ou PDF ;
   un autre format est refusé).
   → Puis « Consulter » : le document s'ouvre par un lien qui **expire en une
   minute**. Rechargez l'onglet une minute plus tard : il est mort.
3. **« Exporter ses données ».**
   → Un JSON avec tout ce que l'organisme détient sur la personne, y compris
   les notes internes.
4. **« Anonymiser »**, avec un motif.
   → L'état civil disparaît, la fiche devient un pseudonyme stable, les champs
   se verrouillent — et les documents scellés **subsistent**. Vérifiez le
   dossier lié : la convention est toujours là. C'est le point qui distingue
   l'anonymisation de la suppression.

---

## 5. Le parcours de l'équipe lemlearn

*Compte : `marie@vulcain.fr`, promu par `LEMLEARN_SUPERADMINS`.*

1. **`/abonnement`** — la formule vue par le client, avec sa consommation face
   aux quotas.
2. **`/admin`** — la même chose vue de l'autre côté : revenu mensuel,
   organisations, dépassements.
3. **Changez la formule** d'une organisation dans le sélecteur.
   → Le revenu mensuel en tête se met à jour ; le changement est journalisé sur
   la chaîne de l'organisation.
4. **« Ouvrir la session »** sur l'autre organisation.
   → Vous basculez dans *ses* données, et un bandeau orange s'affiche dans sa
   barre latérale : « Session ouverte au nom de cet organisme par l'équipe
   lemlearn ». Une impersonation ne peut pas être discrète.

---

## Ce qui ne marchera pas — c'est normal

- **Le paiement en ligne** : sans clés Stripe, `/abonnement` renvoie vers un
  contact au lieu d'un bouton qui échouerait.
- **La relance à froid partie d'elle-même** : la règle quotidienne tourne à 7 h
  UTC ; personne ne l'a encore vue s'exécuter en conditions réelles.
- **Les liens des courriels** pointent vers `dev.lemlearn.fr`, qui n'existe pas.
  En local, remplacez le domaine par `localhost:3000` ; en déployant le front,
  posez `LEMLEARN_APP_URL`.
- **Les contacts `Nour Belkacem 18xxxx`** sont les résidus de mes parcours
  automatisés. Il n'y a pas encore de suppression.
