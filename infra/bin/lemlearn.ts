#!/usr/bin/env node
import { existsSync } from "node:fs";
import { join } from "node:path";
import { App } from "aws-cdk-lib";
import { ComputeStack } from "../lib/compute-stack";
import { DataStack } from "../lib/data-stack";
import { WebStack } from "../lib/web-stack";

const app = new App();

const envName = app.node.tryGetContext("env") ?? process.env.LEMLEARN_ENV ?? "dev";
const isProd = envName === "prod";

const env = {
  account: process.env.CDK_DEFAULT_ACCOUNT,
  // Les données des apprenants restent en France : c'est un argument de vente
  // autant qu'une exigence.
  region: process.env.CDK_DEFAULT_REGION ?? "eu-west-3",
};

const appUrl =
  process.env.LEMLEARN_APP_URL ??
  (isProd ? "https://app.lemlearn.fr" : `https://${envName}.lemlearn.fr`);

const data = new DataStack(app, `Lemlearn-Data-${envName}`, {
  env,
  envName,
  retain: isProd,
  // Les pièces d'identité se déposent depuis le navigateur : une origine
  // oubliée donne un bouton qui tourne sans jamais aboutir, et le message
  // d'erreur du navigateur ne nomme que S3, pas la règle qui manque.
  // LEMLEARN_APP_ORIGINS permet d'en ajouter sans toucher au code.
  appOrigins: [
    ...new Set([
      appUrl,
      ...(process.env.LEMLEARN_APP_ORIGINS ?? "")
        .split(",")
        .map((origin) => origin.trim())
        .filter(Boolean),
      // Hors production : le poste du développeur — Next bascule sur 3001 si
      // 3000 est pris.
      ...(isProd ? [] : ["http://localhost:3000", "http://localhost:3001"]),
    ]),
  ],
  cloudFrontPublicKey: process.env.LEMLEARN_CDN_PUBLIC_KEY,
  description: "lemlearn — tables DynamoDB et compartiments S3",
});

const api = new ComputeStack(app, `Lemlearn-Compute-${envName}`, {
  env,
  envName,
  table: data.table,
  auditTable: data.auditTable,
  documentsBucket: data.documentsBucket,
  identityBucket: data.identityBucket,
  videoBucket: data.videoBucket,
  assetsBucket: data.assetsBucket,
  // Elle sert aux liens des courriels — signature, satisfaction à froid — et
  // à l'origine autorisée en CORS. Une valeur fausse ici donne des liens qui
  // tombent en 404 chez le signataire, qui est la dernière personne à qui on
  // peut demander de recommencer.
  appUrl,
  // La clé publique et le point d'entrée viennent de l'environnement : la
  // clé privée correspondante n'a rien à faire dans un dépôt, et le point
  // d'entrée MediaConvert est propre à chaque compte.
  videoDomain: data.videoDomain,
  videoKeyPairID: data.videoKeyPairID,
  cloudFrontPrivateKey: process.env.LEMLEARN_CDN_KEY,
  mediaConvertEndpoint: process.env.LEMLEARN_MEDIACONVERT_ENDPOINT,
  superAdmins: process.env.LEMLEARN_SUPERADMINS,
  description: "lemlearn — API Lambda et passerelle HTTP",
});

// L'application elle-même : le serveur Next dans une Lambda, derrière sa
// propre distribution. Elle est déployée à part de l'API — le front se
// redéploie à chaque retouche d'écran, l'API beaucoup moins.
//
// Elle n'est montée que si le paquet a été assemblé (`make lambda` dans
// apps/web) : sans lui, un `cdk deploy --all` échouerait sur un dossier
// absent, y compris quand on ne voulait déployer que l'API.
if (existsSync(join(__dirname, "..", "..", "apps", "web", "dist", "lambda"))) {
  // Le secret qui réserve l'URL de fonction à la distribution. Il n'a pas de
  // valeur par défaut : un secret deviné une fois vaudrait pour tout le monde,
  // et un déploiement sans secret ouvrirait le rendu à qui connaît l'adresse.
  const edgeSecret = process.env.LEMLEARN_EDGE_SECRET;
  if (!edgeSecret) {
    throw new Error("LEMLEARN_EDGE_SECRET est requis pour déployer le front (voir ~/.lemlearn/edge-secret)");
  }

  new WebStack(app, `Lemlearn-Web-${envName}`, {
    env,
    envName,
    apiUrl: process.env.LEMLEARN_API_URL ?? api.api.apiEndpoint,
    apiCookie: process.env.LEMLEARN_API_COOKIE,
    edgeSecret,
    publicUrl: process.env.LEMLEARN_APP_URL,
    description: "lemlearn — application Next hébergée en Lambda",
  });
}
