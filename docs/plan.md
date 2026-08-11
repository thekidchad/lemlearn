# lemlearn — CRM + LMS pour organismes de formation (Lot 1)

## Context

Un organisme de formation français vit ou meurt sur sa capacité à **produire la preuve** : preuve d'assiduité, preuve de signature, preuve d'évaluation, preuve de satisfaction. Les outils existants (CRM généraliste + LMS + Yousign + Drive) laissent ces preuves éparpillées, et l'audit Qualiopi devient un chantier manuel de plusieurs jours.

lemlearn réunit les trois : CRM commercial, LMS vidéo, et **chaîne de preuve horodatée** — avec signature électronique interne (pas de coût par document) conforme eIDAS niveau simple/avancé.

Le lot 1 construit une **tranche verticale complète** plutôt qu'un socle large : prospect → devis → convention signée → module vidéo tracké → questionnaire → émargement → attestation → export ZIP du dossier probatoire. Une fois ce fil rouge en place, élargir le CRM et le catalogue est du travail répétitif à faible risque.

Décisions actées : API Go serverless sur AWS Lambda + DynamoDB, vidéo S3 + MediaConvert + CloudFront signé, Next.js sur Vercel, Resend pour l'email, **Typst pour les PDF** (patterns repris de khwiz), UI sombre premium.

---

## Architecture

```
                    app.lemlearn.fr                api.lemlearn.fr
              ┌──────────────────────┐        ┌─────────────────────────┐
   Browser ──▶│ Next.js 16 (Vercel)  │───────▶│ API Gateway HTTP → Go   │
              │ App Router, RSC      │  JWT   │ Lambda arm64, provided.al2023
              │ cookie httpOnly      │ cookie └───────────┬─────────────┘
              └──────────────────────┘                    │
                                                          ▼
   video.lemlearn.fr                         ┌───────────────────────┐
   ┌────────────────────┐                    │ DynamoDB single-table │
   │ CloudFront (signed │◀── HLS ─── S3 ◀────│ + table AUDIT (WORM)  │
   │ cookies, TTL 15min)│        MediaConvert└───────────┬───────────┘
   └────────────────────┘                                │ Streams
                                                         ▼
                                         Firehose → S3 (parquet) → Athena
```

**Monorepo** (pnpm workspaces + turbo) :

```
lemlearn/
├─ apps/web/            Next.js 16 — landing + app + espace apprenant + super-admin
├─ services/api/        Go 1.24 — un module, handlers découpés par domaine
│   ├─ cmd/api/         binaire Lambda unique (routeur chi)
│   ├─ cmd/doc-preview/ CLI de prévisualisation PDF (cf. khwiz document-typst-preview)
│   ├─ internal/{crm,catalog,lms,quiz,signature,proof,billing,identity}
│   └─ internal/platform/{ddb,s3,mail,doc,tsa,audit}
├─ packages/ui/         design system (tokens + primitives shadcn étendues)
├─ packages/contracts/  OpenAPI 3.1 → types TS + structs Go (source de vérité)
└─ infra/               AWS CDK v2 (TypeScript)
```

**Un seul binaire Lambda** routé par chi, pas une Lambda par endpoint : cold start unique, déploiement atomique, tests locaux triviaux (`go run ./cmd/api` sert la même API en HTTP). Deux Lambdas séparées uniquement pour les tâches longues : rendu documentaire (layer Typst) et export ZIP (timeout 300 s).

---

## Modèle de données DynamoDB

Table `lemlearn` — single-table, `PK` / `SK`, isolation tenant par préfixe `ORG#`.

| Entité | PK | SK | GSI1PK / GSI1SK |
|---|---|---|---|
| Organisation | `ORG#<id>` | `META` | `TYPE#ORG` / `<createdAt>` |
| Utilisateur (admin/formateur) | `ORG#<id>` | `USER#<uid>` | `EMAIL#<email>` / `USER` |
| Contact (apprenant / entreprise / financeur) | `ORG#<id>` | `CONTACT#<cid>` | `ORG#<id>#KIND#<kind>` / `<lastName>` |
| Dossier (deal + dossier admin) | `ORG#<id>` | `FILE#<fid>` | `ORG#<id>#STAGE#<stage>` / `<updatedAt>` |
| Formation / module | `ORG#<id>` | `COURSE#<cid>[#MOD#<mid>]` | `ORG#<id>#TAG#<tag>` / `<title>` |
| Session | `ORG#<id>` | `SESSION#<sid>` | `ORG#<id>#DATE` / `<startsAt>` |
| Inscription | `ORG#<id>` | `SESSION#<sid>#ENR#<cid>` | `ORG#<id>#LEARNER#<cid>` / `<startsAt>` |
| Agrégat visionnage | `ORG#<id>` | `ENR#<eid>#WATCH#<mid>` | — |
| Questionnaire (versionné) | `ORG#<id>` | `QUIZ#<qid>#V<n>` | `ORG#<id>#QUIZKIND#<kind>` / `<title>` |
| Tentative + réponses | `ORG#<id>` | `ENR#<eid>#ATT#<qid>#<n>` | `ORG#<id>#QUIZ#<qid>` / `<submittedAt>` |
| Document (devis/convention/attestation) | `ORG#<id>` | `DOC#<did>` | `ORG#<id>#FILE#<fid>` / `<createdAt>` |
| Demande de signature | `ORG#<id>` | `SIG#<sid>` | `TOKEN#<hash>` / `SIG` |

- **GSI2** `ORG#<id>#SEARCH` → préfixe nom/email normalisé, pour l'autocomplétion.
- **Table séparée `lemlearn-audit`** : `PK=SUBJECT#<type>#<id>`, `SK=<ts>#<ulid>`, append-only, **chaînage de hash** (`prevHash` + `hash = SHA256(prevHash‖payload)`) → toute altération casse la chaîne. Aucun `UpdateItem`/`DeleteItem` autorisé par IAM policy.
- **Reporting/export** : DynamoDB Streams → Firehose → S3 parquet → Athena. Aucun scan de table en production.

Toute écriture métier passe par `platform/audit.Append(ctx, subject, action, actor, payload)` dans la **même TransactWriteItems** que la mutation — pas d'événement perdu.

> Réserve assumée : DynamoDB est parfait pour les écritures massives (heartbeats vidéo, audit, réponses de quiz) mais fermé aux requêtes ad-hoc. Les GSIs ci-dessus couvrent les accès connus ; le filtre libre multi-critères sur les contacts nécessitera OpenSearch Serverless en lot 2, branché sur les Streams sans toucher au reste.

---

## Génération documentaire — Typst

Reprise directe de l'approche khwiz (`api/internal/service/document`), qui a déjà remplacé chromedp par Typst : compilation en dizaines de millisecondes, binaire statique sans dépendance système, layout typographique déterministe. Chromium n'entre de toute façon pas dans une Lambda zip.

**À porter depuis khwiz** :

| Source khwiz | Usage dans lemlearn |
|---|---|
| `render_quote_typst_pdf.go` → `TypstCompiler` / `TypstZoneCompiler` / `TypstBinaryCompiler` | interface + implémentation CLI, telles quelles |
| `TypstBinaryCompiler.compile` (temp dir 0600, assets, `--font-path`, `RemoveAll` différé) | identique — les temporaires contiennent des données personnelles |
| `render_typst_chrome.go` → `lem_sig_zone` / `lem_sig_mark` / `lem_mention_zone` + label `<sig-zone>` | déclaration des zones de signature **dans le template**, extraites par `typst query --field value --format json` |
| `stamp.go` (pdfcpu `stampPDFImage` / `stampPDFOverlay`) | apposition du tracé de signature PNG et du cartouche de scellement sur le PDF final |
| `fonts.go` (`go:embed` des TTF statiques) | Typst ne rend pas les polices variables de façon fiable → TTF statiques embarquées |
| `cmd/document-typst-preview` | CLI de prévisualisation, indispensable pour itérer sur les gabarits sans passer par l'API |

**Adaptations Lambda** : le binaire Typst et les polices vont dans un **Lambda layer** (montés en `/opt/bin/typst`, `/opt/fonts`), `TYPST_PATH` pointe dessus ; les sources et le PDF sont écrits dans `/tmp` (512 Mo par défaut) puis effacés. Le renderer est une Lambda dédiée (`doc-render`), appelée en synchrone par l'API — un rendu de convention reste sous la seconde.

**Différence majeure avec khwiz** : là où khwiz déclare les zones pour Yousign, lemlearn les consomme **en interne**. Les coordonnées extraites (page 1-based, origine haut-gauche, points PDF) alimentent directement le placement du tracé manuscrit et du cartouche de scellement. Même convention, plus d'intermédiaire payant.

**Documents du lot 1** : devis, convention de formation, programme, feuille d'émargement, relevé de connexion, relevé d'évaluation, attestation de fin de formation, dossier de preuve.

---

## Chaîne de preuve (le cœur du produit)

### Signature électronique interne (eIDAS simple/avancé)

1. **Émission** — token aléatoire 256 bits ; seul son SHA-256 est stocké (`GSI1PK=TOKEN#<hash>`). Lien unique, expiration 7 jours, usage unique.
2. **Authentification forte** — **OTP à 6 chiffres** (validité 10 min, 3 essais) par email (Resend) ou SMS (SNS), sur un contact vérifié en amont.
3. **Consentement éclairé** — PDF affiché intégralement, case de consentement explicite, mention manuscrite guidée sur la `lem_mention_zone` (« Bon pour accord »), tracé de signature (`<canvas>` → path SVG + coordonnées + timings).
4. **Horodatage qualifié** — hash SHA-256 du PDF envoyé à une **TSA RFC 3161** ; le jeton revient signé par un tiers. C'est ce qui donne la date opposable, pas l'heure serveur.
5. **Scellement** — tracé apposé sur les zones extraites (pdfcpu), puis signature PAdES-B-T avec le certificat de l'organisme (clé en **AWS KMS**, jamais en clair) et jeton TSA embarqué. Toute modification ultérieure invalide la signature — vérifiable dans Adobe Reader.
6. **Dossier de preuve** — PDF annexe généré en Typst : IP, user-agent, empreinte appareil, email/téléphone certifiés, horodatage de chaque étape, hash du document, tracé reproduit, extrait de la chaîne d'audit.
7. **Archivage** — PDF signé + dossier de preuve dans **S3 avec Object Lock (mode COMPLIANCE)**, rétention par type de document (conventions : 10 ans). Ni l'admin, ni le compte root ne peuvent supprimer avant expiration.

Bibliothèques Go : `pdfcpu` (déjà utilisé chez khwiz), `digitorus/pdfsign` (PAdES/PKCS#7), `digitorus/timestamp` (RFC 3161).

### Tracking d'assiduité vidéo

- Upload : presigned multipart S3 → EventBridge → MediaConvert (HLS 3 rendus + vignettes) → asset `ready`.
- Lecture : `POST /sessions/{id}/modules/{id}/playback` vérifie l'inscription, pose des **cookies CloudFront signés** (TTL 15 min, scope au chemin de l'asset), retourne le manifest.
- Player `hls.js` maison : **heartbeat toutes les 5 s** avec `{position, playedMs, rate, focused}`. Le serveur ne fait jamais confiance au client : progression monotone et plausible (≤ temps réel × vitesse max), sauts ignorés, lecture en arrière-plan détectée.
- Agrégat par inscription+module : `watchedMs`, `uniqueCoverage` (bitmap des segments de 10 s réellement vus — rejouer 3× la même minute n'accumule pas 3 minutes), `firstAt`, `lastAt`, `deviceCount`.
- Export **relevé de connexion** PDF/CSV : une ligne par séance de visionnage, horodatée, durée cumulée, taux de complétion. C'est la pièce que demande l'auditeur.

### Questionnaires — moteur unique, traçabilité intégrale

Un seul moteur couvre tous les usages, distingués par `kind` : `positionnement` (entrée), `post_module` (**après chaque vidéo**), `intermediaire`, `final`, `satisfaction_chaud`, `satisfaction_froid`.

**Types de question** : choix unique, choix multiple, vrai/faux, échelle Likert 1–5, numérique, texte libre, appariement. Chaque question porte son barème, son corrigé et une explication affichée après coup (le post-module est formatif : on explique la bonne réponse).

**Versionnement obligatoire** : un questionnaire modifié crée `QUIZ#<qid>#V<n+1>`. Une tentative référence sa version — on doit pouvoir prouver *ce qui a été demandé* le jour J, pas ce que le questionnaire est devenu depuis.

**Ce qui est enregistré, par tentative** :

```
ATT#<quizId>#<n>  { version, startedAt, submittedAt, durationMs,
                    score, maxScore, passed, attemptNumber,
                    answers: [ { questionId, value, isCorrect, points,
                                 timeSpentMs, changeCount, answeredAt } ],
                    ip, userAgent, moduleId, watchedMsAtStart }
```

`changeCount` et `timeSpentMs` par question ne servent pas qu'à la statistique : ils distinguent un apprenant qui réfléchit d'un questionnaire cliqué en huit secondes, et alimentent la mesure des acquis. Chaque soumission est un événement de la chaîne d'audit ; le brut part aussi vers Firehose pour l'analyse par question (taux de réussite, distracteurs jamais choisis).

**Verrouillage de progression** : un module est *complété* si `uniqueCoverage ≥ seuil` **et** `quiz post_module réussi ≥ seuil`. La progression du parcours en découle, et donc l'attestation. C'est ce couplage assiduité × acquis qui fait la solidité du dossier Qualiopi.

**Rendu anti-triche léger** : ordre des questions et des options mélangé par tentative (graine dérivée de l'inscription, donc reproductible pour l'audit), nombre de tentatives paramétrable, temps limité optionnel, corrigé révélé seulement après soumission.

**Satisfaction à froid** : à la clôture de session, une **EventBridge Scheduler** programme l'envoi à M+3 (paramétrable). Relance à J+7 et J+14 si sans réponse. Le taux de retour est affiché dans le tableau de bord Qualiopi — c'est un indicateur audité.

**Relevé d'évaluation** (Typst) : une page par tentative avec questions, réponses données, corrigé, score, horodatage — pièce jointe à l'export du dossier.

### Émargement numérique

Par créneau (demi-journée en présentiel, par module en asynchrone) : l'apprenant signe depuis son espace ou via un lien envoyé le matin. En asynchrone, l'émargement est **pré-rempli par les données de visionnage et de quiz** et contresigné par le formateur. Feuille de présence générée en fin de session : liste, créneaux, signatures, horodatages, signature formateur, scellement identique aux conventions.

### Export du dossier

`POST /files/{id}/export` → Lambda dédiée (timeout 300 s) assemblant un ZIP : fiche apprenant complète (nom, prénom, date de naissance, adresse, email, téléphone, pièce d'identité), devis, convention signée + dossier de preuve, programme, relevés de connexion, relevés d'évaluation (toutes tentatives, tous questionnaires), feuilles d'émargement, satisfaction chaud et froid, attestation, `manifest.json` (SHA-256 de chaque pièce) et `audit-trail.csv`. Livré par URL S3 presigned, expiration 1 h ; l'export lui-même est audité.

---

## Conformité RGPD

- **Consentement** versionné et horodaté (finalité, version des CGU, date, IP), stocké comme événement d'audit.
- **Pièce d'identité** : bucket S3 dédié, chiffré KMS avec clé par tenant, accès uniquement par URL presigned de 60 s, jamais servi en direct. Suppression automatique après validation du dossier (recommandation CNIL).
- **Droit à l'effacement** : anonymisation plutôt que suppression — les pièces probatoires doivent survivre à la durée légale. Le contact devient `Apprenant anonymisé #<hash>`, les PII sont purgées, les documents scellés restent sous Object Lock avec un pseudonyme dans l'index.
- **Portabilité** : `GET /me/export` → ZIP JSON + pièces.
- **Rétention** : TTL DynamoDB sur le non-probatoire (heartbeats bruts : 13 mois), Object Lock sur le probatoire, registre des traitements généré depuis la config.

---

## Design — sombre premium

Direction : dense mais respirante, contraste maîtrisé, **une seule couleur d'accent**, mouvement rare et rapide. Ce n'est pas un dashboard de démo : c'est un outil consulté 8 h/jour par une assistante de formation, donc la lisibilité prime sur l'effet.

**Tokens** (`packages/ui/tokens.css`) — palette complète définie sur `:root`, thème sombre par défaut via `data-theme` :

```
surface-0  #0B0C0F   fond application
surface-1  #121318   cartes, panneaux
surface-2  #1A1C23   élévation, popovers
border     #24262F / hover #32353F
text-1     #EDEEF2   text-2 #A0A4B0   text-3 #6B6F7D
accent     #7C5CFF   hover #8F73FF, ring 40 % alpha
success    #34D399   warn #FBBF24   danger #F87171
statut de preuve : complète #34D399 · partielle #FBBF24 · manquante #F87171
```

Typographie : **Geist** (déjà en TTF statiques côté Typst — mêmes polices à l'écran et sur le PDF, cohérence gratuite). Échelle 12 / 13 / 15 / 20 / 28 / 40, `-5 %` de letter-spacing sur les titres, `tabular-nums` sur tous les chiffres. Rayons 6 / 10 / 14. Transitions 120 ms `cubic-bezier(.2,.8,.2,1)`, jamais plus de 200 ms.

**Écrans du lot 1** (maquettes générées avec le MCP `uiux-image-gen` avant implémentation) :

1. **Landing** — hero « Votre audit Qualiopi, déjà prêt », démonstration animée de la chaîne de preuve, section conformité, pricing 3 paliers, FAQ.
2. **Pipeline** — kanban drag & drop, colonnes Prospect / Devis / Convention / En formation / Clôturé, carte compacte avec pastille de complétude du dossier.
3. **Fiche dossier** — l'écran signature du produit : timeline verticale à gauche (chaque événement horodaté), onglets Identité / Documents / Formation / Preuves à droite, bouton « Exporter le dossier » toujours visible, jauge de complétude Qualiopi en tête.
4. **Signature** (public, mobile-first) — visionneuse PDF, OTP, mention guidée, canvas de signature, confirmation avec le hash affiché.
5. **Lecteur apprenant** — vidéo plein cadre, barre de progression doublée d'une **piste de couverture réelle** (segments vus en accent, non vus en gris), ressources sous le lecteur, **questionnaire qui se déplie à la fin de la vidéo** sans changer de page.
6. **Éditeur de questionnaire** — colonne de questions réordonnables, édition inline, aperçu apprenant en direct, réglages (seuil, tentatives, mélange) en panneau latéral.
7. **Résultats de questionnaire** — vue par apprenant (tentatives, temps, réponses) et vue par question (taux de réussite, distracteurs) pour améliorer la formation.
8. **Émargement** — grille créneaux × apprenants, cellules signées / en attente / absent, signature formateur en pied.
9. **Super-admin** — organisations, MRR, usage (stockage, minutes vidéo, signatures), impersonation tracée, abonnements Stripe.

Raccourcis dès le lot 1 : `⌘K` palette de commandes, `G` puis `D/S/C` pour naviguer, `N` nouveau. Une palette de commandes bien faite est ce qui fait passer un CRM de « correct » à « rapide ».

---

## Découpage d'exécution

| # | Étape | Livrable vérifiable |
|---|---|---|
| 1 | Monorepo, CDK (DynamoDB, S3 ×3, CloudFront, API GW, Lambda, layer Typst), CI | `cdk deploy` en `dev`, `GET /health` répond |
| 2 | Design system + tokens + maquettes MCP | Storybook des primitives, 9 maquettes validées |
| 3 | Identité : orgs, users, rôles (owner/admin/formateur/apprenant/superadmin), JWT cookie, argon2id | Inscription → login → isolation tenant vérifiée |
| 4 | Contacts + dossiers + pipeline + audit chaîné | Créer un dossier, le déplacer, lire la timeline |
| 5 | Catalogue : formations, modules, sessions, tags, calendrier | Formation 2 modules, session, inscription |
| 6 | Socle Typst porté de khwiz + gabarits devis/convention/programme + CLI de preview | PDF conformes, zones extraites par `typst query` |
| 7 | Signature : token, OTP Resend, mention guidée, canvas, TSA, PAdES/KMS, Object Lock | Convention signée + dossier de preuve, validée dans Adobe |
| 8 | Vidéo : upload, MediaConvert, CloudFront signé, player, heartbeats, couverture | Relevé de connexion exportable |
| 9 | Moteur de questionnaires : modèle versionné, éditeur, passation, correction, verrouillage de progression, vues de résultats | Quiz après chaque vidéo, toutes réponses tracées |
| 10 | Émargement + attestation + satisfaction chaud/froid (EventBridge Scheduler) | Feuilles signées, relance à froid planifiée à M+3 |
| 11 | Export ZIP + manifest + audit-trail | Dossier complet téléchargé et relu |
| 12 | Super-admin + Stripe (abonnements, quotas, impersonation tracée) | Abonner un OF, voir son usage |
| 13 | RGPD : consentements, anonymisation, portabilité, TTL | Anonymiser un apprenant sans casser les scellés |
| 14 | Landing + SEO + Resend transactionnel | lemlearn.fr en production |

---

## Vérification

- **Go** : tests unitaires sur la logique d'assiduité (sauts, accélération, replay), le chaînage de hash de l'audit, la correction et le barème des questionnaires, les règles de rétention. `dynamodb-local` en Docker pour l'intégration, pas de mock maison.
- **Typst** : tests de rendu façon khwiz (`render_*_typst_test.go`) — compilation réelle du gabarit et assertions sur le texte extrait et les zones retournées ; un gabarit signable sans `lem_sig_zone` doit échouer le test.
- **Chaîne de signature** : test end-to-end qui génère une convention, signe, puis vérifie le PDF avec `pdfsig` (poppler) — signature valide, jeton TSA présent, et **altération d'un octet ⇒ vérification en échec**. C'est le test qui protège la valeur juridique du produit.
- **Vidéo** : simulation d'un apprenant regardant 3× la même minute → l'assiduité doit rester à 1 minute de couverture unique.
- **Questionnaires** : test qui vérifie qu'une tentative reste rattachée à la version V1 après publication de V2, et que le relevé d'évaluation réimprime bien les questions de V1.
- **Object Lock** : `DeleteObject` sur une convention scellée doit renvoyer `AccessDenied`.
- **Front** : Playwright sur les deux parcours critiques (admin : dossier → convention → export ; apprenant : lien OTP → signature → vidéo → questionnaire → attestation). Axe-core sur les 9 écrans, contraste AA minimum en thème sombre.
- **Recette manuelle** : un dossier complet exporté, ouvert, relu pièce par pièce comme le ferait un auditeur Qualiopi — seul vrai critère d'acceptation du lot 1.

---

## Points à trancher plus tard (non bloquants)

- **Certificat de signature** : un certificat de cachet d'organisation (Certigna, ChamberSign, ~300 €/an) est nécessaire pour le PAdES en production. Auto-signé en dev, à commander pendant l'étape 6.
- **TSA** : FreeTSA en dev ; en production, une TSA qualifiée eIDAS (Certigna, Universign) est plus solide face à un financeur.
- **SMS OTP** : SNS convient pour la France ; l'email suffit en eIDAS simple, le SMS renforce vers l'avancé.
- **Recherche libre** sur les contacts : OpenSearch Serverless si les GSIs deviennent insuffisants.
- **Banque de questions** partagée entre formations (réutilisation, tirage aléatoire dans un pool) : prévue au modèle, non implémentée en lot 1.
