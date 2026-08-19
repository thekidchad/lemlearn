#!/usr/bin/env node
import { App } from "aws-cdk-lib";
import { ComputeStack } from "../lib/compute-stack";
import { DataStack } from "../lib/data-stack";

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
  // L'application déployée, et le poste du développeur hors production : les
  // pièces d'identité se déposent depuis le navigateur, et une origine
  // oubliée donne un bouton qui tourne sans jamais aboutir.
  appOrigins: isProd ? [appUrl] : [appUrl, "http://localhost:3000"],
  cloudFrontPublicKey: process.env.LEMLEARN_CDN_PUBLIC_KEY,
  description: "lemlearn — tables DynamoDB et compartiments S3",
});

new ComputeStack(app, `Lemlearn-Compute-${envName}`, {
  env,
  envName,
  table: data.table,
  auditTable: data.auditTable,
  documentsBucket: data.documentsBucket,
  identityBucket: data.identityBucket,
  videoBucket: data.videoBucket,
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
