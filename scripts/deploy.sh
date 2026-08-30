#!/usr/bin/env bash
#
# Déploiement complet de lemlearn sur un compte AWS.
#
# Tout passe par des variables d'environnement, et c'est là qu'est le piège :
# CDK les lit au synthé, donc une variable absente n'est pas « inchangée »,
# elle est « effacée ». Un déploiement qui oublie LEMLEARN_SUPERADMINS retire
# l'accès de l'équipe ; un qui oublie la clé de signature coupe la vidéo. Ce
# script les rassemble en un seul endroit et refuse de partir s'il en manque
# une, plutôt que de laisser découvrir la panne en production.
#
#   scripts/deploy.sh                 # tout, environnement dev
#   scripts/deploy.sh prod            # tout, environnement prod
#   scripts/deploy.sh dev web         # seulement le front
#   scripts/deploy.sh dev api         # seulement l'API et les données
#
# La configuration se met dans scripts/.env.<environnement> (ignoré par git) —
# voir scripts/env.example. Les secrets qui ne doivent pas y figurer, parce
# qu'ils survivent aux environnements, restent dans un dossier à part :
# ~/.lemlearn par défaut, ou $LEMLEARN_SECRETS_DIR.

set -euo pipefail

ENVNAME="${1:-dev}"
CIBLE="${2:-tout}"
RACINE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SECRETS="${LEMLEARN_SECRETS_DIR:-$HOME/.lemlearn}"

info() { printf '\n\033[1m%s\033[0m\n' "$*"; }
erreur() { printf '\033[31m%s\033[0m\n' "$*" >&2; }

# ---------------------------------------------------------------- configuration

# Le fichier racine porte les secrets qui ne dépendent pas de l'environnement —
# clés de fournisseurs, identifiants. Il est chargé en premier pour que la
# configuration de l'environnement puisse le surcharger, jamais l'inverse.
if [ -f "$RACINE/.env" ]; then
  # shellcheck disable=SC1091
  set -a && . "$RACINE/.env" && set +a
fi

FICHIER="$RACINE/scripts/.env.$ENVNAME"
if [ -f "$FICHIER" ]; then
  # shellcheck disable=SC1090
  set -a && . "$FICHIER" && set +a
  info "configuration : scripts/.env.$ENVNAME"
else
  info "configuration : environnement courant (scripts/.env.$ENVNAME absent)"
fi

: "${AWS_PROFILE:?AWS_PROFILE est requis}"
: "${AWS_REGION:=eu-west-3}"
export AWS_PROFILE AWS_REGION

# ------------------------------------------------------------------- secrets
#
# Ces deux-là ne sont pas dans le dépôt et ne se régénèrent pas sans casser
# quelque chose : la clé privée signe les URL vidéo, sa jumelle publique étant
# déposée dans la distribution ; le secret de bordure réserve le rendu à
# CloudFront. On les fabrique s'ils n'existent pas — c'est le cas d'un compte
# neuf — et on ne les touche plus jamais ensuite.

mkdir -p "$SECRETS" && chmod 700 "$SECRETS"

if [ ! -f "$SECRETS/cloudfront.pem" ]; then
  info "clé de signature vidéo absente : génération"
  openssl genrsa -out "$SECRETS/cloudfront.pem" 2048 2>/dev/null
  openssl rsa -pubout -in "$SECRETS/cloudfront.pem" -out "$SECRETS/cloudfront.pub" 2>/dev/null
  chmod 600 "$SECRETS/cloudfront.pem"
fi

if [ ! -f "$SECRETS/edge-secret" ]; then
  info "secret de bordure absent : génération"
  openssl rand -hex 32 > "$SECRETS/edge-secret"
  chmod 600 "$SECRETS/edge-secret"
fi

export LEMLEARN_CDN_KEY="$(cat "$SECRETS/cloudfront.pem")"
export LEMLEARN_CDN_PUBLIC_KEY="$(cat "$SECRETS/cloudfront.pub")"
export LEMLEARN_EDGE_SECRET="$(cat "$SECRETS/edge-secret")"

if [ -z "${LEMLEARN_SUPERADMINS:-}" ]; then
  erreur "LEMLEARN_SUPERADMINS n'est pas défini : personne ne pourra ouvrir la vue"
  erreur "super-admin sur un compte neuf. Renseignez-le dans scripts/.env.$ENVNAME."
fi

# ------------------------------------------------------------------- compilation

if [ "$CIBLE" = "tout" ] || [ "$CIBLE" = "api" ]; then
  info "compilation de l'API"
  make -C "$RACINE/services/api" lambda
  # Le layer embarque Typst et les polices : il ne change qu'à la montée de
  # version, et son téléchargement dure plus longtemps que tout le reste.
  [ -d "$RACINE/services/api/dist/layer/bin" ] || make -C "$RACINE/services/api" layer
fi

if [ "$CIBLE" = "tout" ] || [ "$CIBLE" = "web" ]; then
  info "assemblage du front"
  make -C "$RACINE/apps/web" lambda
fi

# -------------------------------------------------------------------- piles

case "$CIBLE" in
  api) PILES=(Lemlearn-Data-"$ENVNAME" Lemlearn-Compute-"$ENVNAME") ;;
  web) PILES=(Lemlearn-Web-"$ENVNAME") ;;
  *)   PILES=(--all) ;;
esac

# Un compte AWS neuf n'a pas de quoi recevoir un déploiement CDK : il lui
# manque le compartiment d'artefacts et les rôles d'exécution. L'amorçage est
# idempotent, donc on le tente à chaque fois plutôt que de demander à
# quelqu'un de s'en souvenir une seule fois, le jour où ça compte.
amorcer() {
  local compte
  compte="$(aws sts get-caller-identity --query Account --output text 2>/dev/null || true)"
  if [ -z "$compte" ]; then
    erreur "identifiants AWS invalides pour le profil $AWS_PROFILE"
    exit 1
  fi
  info "compte $compte, région $AWS_REGION"

  # La distribution du front vit à Paris comme le reste, mais son certificat —
  # le jour où un domaine propre sera branché — devra être émis en us-east-1.
  # On amorce les deux régions maintenant : y revenir plus tard demanderait de
  # comprendre pourquoi ACM refuse.
  # L'échec n'est pas bloquant : sur un compte déjà amorcé, il signifie le
  # plus souvent qu'on n'a pas le droit de *vérifier* l'amorçage — ce qui
  # n'empêche pas de déployer. Un compte réellement vierge échouera de toute
  # façon au déploiement, avec un message qui nomme ce qui manque.
  if (cd "$RACINE/infra" && npx cdk bootstrap \
      "aws://$compte/$AWS_REGION" "aws://$compte/us-east-1" >/dev/null 2>&1); then
    info "amorçage vérifié"
  else
    erreur "amorçage non vérifiable (droits insuffisants) — on continue"
    erreur "sur un compte neuf, lancez : npx cdk bootstrap aws://$compte/$AWS_REGION aws://$compte/us-east-1"
  fi
}

amorcer

deployer() {
  info "déploiement : ${PILES[*]}"
  (cd "$RACINE/infra" && npx cdk deploy "${PILES[@]}" \
    --context env="$ENVNAME" --require-approval never --outputs-file /tmp/lemlearn-sorties.json)
}

# Sur un compte neuf, l'adresse du front n'existe pas encore : le premier
# passage la crée, le second la transmet. Sans elle, Next croit servir
# 127.0.0.1 et refuse toutes les actions de formulaire — connexion comprise —
# et les liens des courriels pointent dans le vide.
deployer

TROUVEE="$(python3 - <<'PY'
import json, sys
try:
    sorties = json.load(open("/tmp/lemlearn-sorties.json"))
except Exception:
    sys.exit()
for pile in sorties.values():
    for cle, valeur in pile.items():
        if cle == "WebUrl":
            print(valeur)
PY
)"

if [ -n "$TROUVEE" ] && [ "${LEMLEARN_APP_URL:-}" != "$TROUVEE" ]; then
  info "adresse du front : $TROUVEE — second passage pour la propager"
  export LEMLEARN_APP_URL="$TROUVEE"
  deployer
fi

info "terminé"
[ -f /tmp/lemlearn-sorties.json ] && python3 -m json.tool /tmp/lemlearn-sorties.json
