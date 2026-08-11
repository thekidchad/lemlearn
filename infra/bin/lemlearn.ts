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

const data = new DataStack(app, `Lemlearn-Data-${envName}`, {
  env,
  envName,
  retain: isProd,
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
  appUrl: isProd ? "https://app.lemlearn.fr" : `https://${envName}.lemlearn.fr`,
  description: "lemlearn — API Lambda et passerelle HTTP",
});
