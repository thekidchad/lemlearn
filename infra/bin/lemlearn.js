#!/usr/bin/env node
"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const node_fs_1 = require("node:fs");
const node_path_1 = require("node:path");
const aws_cdk_lib_1 = require("aws-cdk-lib");
const compute_stack_1 = require("../lib/compute-stack");
const data_stack_1 = require("../lib/data-stack");
const web_stack_1 = require("../lib/web-stack");
const app = new aws_cdk_lib_1.App();
const envName = app.node.tryGetContext("env") ?? process.env.LEMLEARN_ENV ?? "dev";
const isProd = envName === "prod";
const env = {
    account: process.env.CDK_DEFAULT_ACCOUNT,
    // Les données des apprenants restent en France : c'est un argument de vente
    // autant qu'une exigence.
    region: process.env.CDK_DEFAULT_REGION ?? "eu-west-3",
};
const appUrl = process.env.LEMLEARN_APP_URL ??
    (isProd ? "https://app.lemlearn.fr" : `https://${envName}.lemlearn.fr`);
const data = new data_stack_1.DataStack(app, `Lemlearn-Data-${envName}`, {
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
const api = new compute_stack_1.ComputeStack(app, `Lemlearn-Compute-${envName}`, {
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
if ((0, node_fs_1.existsSync)((0, node_path_1.join)(__dirname, "..", "..", "apps", "web", "dist", "lambda"))) {
    // Le secret qui réserve l'URL de fonction à la distribution. Il n'a pas de
    // valeur par défaut : un secret deviné une fois vaudrait pour tout le monde,
    // et un déploiement sans secret ouvrirait le rendu à qui connaît l'adresse.
    const edgeSecret = process.env.LEMLEARN_EDGE_SECRET;
    if (!edgeSecret) {
        throw new Error("LEMLEARN_EDGE_SECRET est requis pour déployer le front (voir ~/.lemlearn/edge-secret)");
    }
    new web_stack_1.WebStack(app, `Lemlearn-Web-${envName}`, {
        env,
        envName,
        apiUrl: process.env.LEMLEARN_API_URL ?? api.api.apiEndpoint,
        apiCookie: process.env.LEMLEARN_API_COOKIE,
        edgeSecret,
        publicUrl: process.env.LEMLEARN_APP_URL,
        description: "lemlearn — application Next hébergée en Lambda",
    });
}
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJmaWxlIjoibGVtbGVhcm4uanMiLCJzb3VyY2VSb290IjoiIiwic291cmNlcyI6WyJsZW1sZWFybi50cyJdLCJuYW1lcyI6W10sIm1hcHBpbmdzIjoiOzs7QUFDQSxxQ0FBcUM7QUFDckMseUNBQWlDO0FBQ2pDLDZDQUFrQztBQUNsQyx3REFBb0Q7QUFDcEQsa0RBQThDO0FBQzlDLGdEQUE0QztBQUU1QyxNQUFNLEdBQUcsR0FBRyxJQUFJLGlCQUFHLEVBQUUsQ0FBQztBQUV0QixNQUFNLE9BQU8sR0FBRyxHQUFHLENBQUMsSUFBSSxDQUFDLGFBQWEsQ0FBQyxLQUFLLENBQUMsSUFBSSxPQUFPLENBQUMsR0FBRyxDQUFDLFlBQVksSUFBSSxLQUFLLENBQUM7QUFDbkYsTUFBTSxNQUFNLEdBQUcsT0FBTyxLQUFLLE1BQU0sQ0FBQztBQUVsQyxNQUFNLEdBQUcsR0FBRztJQUNWLE9BQU8sRUFBRSxPQUFPLENBQUMsR0FBRyxDQUFDLG1CQUFtQjtJQUN4Qyw0RUFBNEU7SUFDNUUsMEJBQTBCO0lBQzFCLE1BQU0sRUFBRSxPQUFPLENBQUMsR0FBRyxDQUFDLGtCQUFrQixJQUFJLFdBQVc7Q0FDdEQsQ0FBQztBQUVGLE1BQU0sTUFBTSxHQUNWLE9BQU8sQ0FBQyxHQUFHLENBQUMsZ0JBQWdCO0lBQzVCLENBQUMsTUFBTSxDQUFDLENBQUMsQ0FBQyx5QkFBeUIsQ0FBQyxDQUFDLENBQUMsV0FBVyxPQUFPLGNBQWMsQ0FBQyxDQUFDO0FBRTFFLE1BQU0sSUFBSSxHQUFHLElBQUksc0JBQVMsQ0FBQyxHQUFHLEVBQUUsaUJBQWlCLE9BQU8sRUFBRSxFQUFFO0lBQzFELEdBQUc7SUFDSCxPQUFPO0lBQ1AsTUFBTSxFQUFFLE1BQU07SUFDZCx1RUFBdUU7SUFDdkUsd0VBQXdFO0lBQ3hFLG1FQUFtRTtJQUNuRSxpRUFBaUU7SUFDakUsVUFBVSxFQUFFO1FBQ1YsR0FBRyxJQUFJLEdBQUcsQ0FBQztZQUNULE1BQU07WUFDTixHQUFHLENBQUMsT0FBTyxDQUFDLEdBQUcsQ0FBQyxvQkFBb0IsSUFBSSxFQUFFLENBQUM7aUJBQ3hDLEtBQUssQ0FBQyxHQUFHLENBQUM7aUJBQ1YsR0FBRyxDQUFDLENBQUMsTUFBTSxFQUFFLEVBQUUsQ0FBQyxNQUFNLENBQUMsSUFBSSxFQUFFLENBQUM7aUJBQzlCLE1BQU0sQ0FBQyxPQUFPLENBQUM7WUFDbEIsdUVBQXVFO1lBQ3ZFLGlCQUFpQjtZQUNqQixHQUFHLENBQUMsTUFBTSxDQUFDLENBQUMsQ0FBQyxFQUFFLENBQUMsQ0FBQyxDQUFDLENBQUMsdUJBQXVCLEVBQUUsdUJBQXVCLENBQUMsQ0FBQztTQUN0RSxDQUFDO0tBQ0g7SUFDRCxtQkFBbUIsRUFBRSxPQUFPLENBQUMsR0FBRyxDQUFDLHVCQUF1QjtJQUN4RCxXQUFXLEVBQUUsZ0RBQWdEO0NBQzlELENBQUMsQ0FBQztBQUVILE1BQU0sR0FBRyxHQUFHLElBQUksNEJBQVksQ0FBQyxHQUFHLEVBQUUsb0JBQW9CLE9BQU8sRUFBRSxFQUFFO0lBQy9ELEdBQUc7SUFDSCxPQUFPO0lBQ1AsS0FBSyxFQUFFLElBQUksQ0FBQyxLQUFLO0lBQ2pCLFVBQVUsRUFBRSxJQUFJLENBQUMsVUFBVTtJQUMzQixlQUFlLEVBQUUsSUFBSSxDQUFDLGVBQWU7SUFDckMsY0FBYyxFQUFFLElBQUksQ0FBQyxjQUFjO0lBQ25DLFdBQVcsRUFBRSxJQUFJLENBQUMsV0FBVztJQUM3QixZQUFZLEVBQUUsSUFBSSxDQUFDLFlBQVk7SUFDL0IsMkVBQTJFO0lBQzNFLDJFQUEyRTtJQUMzRSwyRUFBMkU7SUFDM0UsZ0NBQWdDO0lBQ2hDLE1BQU07SUFDTix3RUFBd0U7SUFDeEUsd0VBQXdFO0lBQ3hFLG9EQUFvRDtJQUNwRCxXQUFXLEVBQUUsSUFBSSxDQUFDLFdBQVc7SUFDN0IsY0FBYyxFQUFFLElBQUksQ0FBQyxjQUFjO0lBQ25DLG9CQUFvQixFQUFFLE9BQU8sQ0FBQyxHQUFHLENBQUMsZ0JBQWdCO0lBQ2xELG9CQUFvQixFQUFFLE9BQU8sQ0FBQyxHQUFHLENBQUMsOEJBQThCO0lBQ2hFLFdBQVcsRUFBRSxPQUFPLENBQUMsR0FBRyxDQUFDLG9CQUFvQjtJQUM3QyxXQUFXLEVBQUUsMENBQTBDO0NBQ3hELENBQUMsQ0FBQztBQUVILHlFQUF5RTtBQUN6RSx1RUFBdUU7QUFDdkUsNkRBQTZEO0FBQzdELEVBQUU7QUFDRix3RUFBd0U7QUFDeEUsd0VBQXdFO0FBQ3hFLDREQUE0RDtBQUM1RCxJQUFJLElBQUEsb0JBQVUsRUFBQyxJQUFBLGdCQUFJLEVBQUMsU0FBUyxFQUFFLElBQUksRUFBRSxJQUFJLEVBQUUsTUFBTSxFQUFFLEtBQUssRUFBRSxNQUFNLEVBQUUsUUFBUSxDQUFDLENBQUMsRUFBRSxDQUFDO0lBQzdFLDJFQUEyRTtJQUMzRSw2RUFBNkU7SUFDN0UsNEVBQTRFO0lBQzVFLE1BQU0sVUFBVSxHQUFHLE9BQU8sQ0FBQyxHQUFHLENBQUMsb0JBQW9CLENBQUM7SUFDcEQsSUFBSSxDQUFDLFVBQVUsRUFBRSxDQUFDO1FBQ2hCLE1BQU0sSUFBSSxLQUFLLENBQUMsdUZBQXVGLENBQUMsQ0FBQztJQUMzRyxDQUFDO0lBRUQsSUFBSSxvQkFBUSxDQUFDLEdBQUcsRUFBRSxnQkFBZ0IsT0FBTyxFQUFFLEVBQUU7UUFDM0MsR0FBRztRQUNILE9BQU87UUFDUCxNQUFNLEVBQUUsT0FBTyxDQUFDLEdBQUcsQ0FBQyxnQkFBZ0IsSUFBSSxHQUFHLENBQUMsR0FBRyxDQUFDLFdBQVc7UUFDM0QsU0FBUyxFQUFFLE9BQU8sQ0FBQyxHQUFHLENBQUMsbUJBQW1CO1FBQzFDLFVBQVU7UUFDVixTQUFTLEVBQUUsT0FBTyxDQUFDLEdBQUcsQ0FBQyxnQkFBZ0I7UUFDdkMsV0FBVyxFQUFFLGdEQUFnRDtLQUM5RCxDQUFDLENBQUM7QUFDTCxDQUFDIn0=