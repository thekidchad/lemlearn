"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.ComputeStack = void 0;
const aws_cdk_lib_1 = require("aws-cdk-lib");
const apigw = require("aws-cdk-lib/aws-apigatewayv2");
const integrations = require("aws-cdk-lib/aws-apigatewayv2-integrations");
const iam = require("aws-cdk-lib/aws-iam");
const lambda = require("aws-cdk-lib/aws-lambda");
const events = require("aws-cdk-lib/aws-events");
const targets = require("aws-cdk-lib/aws-events-targets");
const logs = require("aws-cdk-lib/aws-logs");
const path = require("node:path");
/**
 * Calcul : une Lambda unique sert toute l'API, routée par chi côté Go.
 *
 * Une fonction par endpoint multiplierait les démarrages à froid et les
 * déploiements partiels ; ici un seul artefact, une seule version, un seul
 * rollback.
 */
class ComputeStack extends aws_cdk_lib_1.Stack {
    api;
    constructor(scope, id, props) {
        super(scope, id, props);
        const apiRoot = path.join(__dirname, "..", "..", "services", "api");
        // Le binaire Typst et les polices sont montés en lecture seule sous
        // /opt : les extraire dans /tmp à chaque invocation coûterait un
        // démarrage à froid inutile. `make layer` prépare le contenu.
        const typstLayer = new lambda.LayerVersion(this, "TypstLayer", {
            code: lambda.Code.fromAsset(path.join(apiRoot, "dist", "layer")),
            compatibleRuntimes: [lambda.Runtime.PROVIDED_AL2023],
            compatibleArchitectures: [lambda.Architecture.ARM_64],
            description: "Binaire Typst et polices Geist statiques",
        });
        const sharedEnv = {
            LEMLEARN_ENV: props.envName,
            LEMLEARN_TABLE: props.table.tableName,
            LEMLEARN_AUDIT_TABLE: props.auditTable.tableName,
            LEMLEARN_DOCUMENTS_BUCKET: props.documentsBucket.bucketName,
            LEMLEARN_IDENTITY_BUCKET: props.identityBucket.bucketName,
            LEMLEARN_VIDEO_BUCKET: props.videoBucket.bucketName,
            LEMLEARN_APP_URL: props.appUrl,
            TYPST_PATH: "/opt/bin/typst",
            TYPST_FONT_PATH: "/opt/fonts",
            LEMLEARN_SUPERADMINS: props.superAdmins ?? "",
            // Envoi de courriels et facturation : absents en recette, où rien ne
            // part et où le journal des envois suffit. On ne pose que ce qui est
            // fourni — une variable vide vaudrait une clé invalide.
            ...(process.env.RESEND_API_KEY ? { RESEND_API_KEY: process.env.RESEND_API_KEY } : {}),
            ...(process.env.LEMLEARN_MAIL_FROM ? { LEMLEARN_MAIL_FROM: process.env.LEMLEARN_MAIL_FROM } : {}),
            ...(process.env.STRIPE_SECRET_KEY ? { STRIPE_SECRET_KEY: process.env.STRIPE_SECRET_KEY } : {}),
            ...(process.env.STRIPE_WEBHOOK_SECRET
                ? { STRIPE_WEBHOOK_SECRET: process.env.STRIPE_WEBHOOK_SECRET }
                : {}),
            ...(process.env.STRIPE_PRICES ? { STRIPE_PRICES: process.env.STRIPE_PRICES } : {}),
            LEMLEARN_ASSETS_URL: `https://${props.assetsBucket.bucketName}.s3.${aws_cdk_lib_1.Stack.of(props.assetsBucket).region}.amazonaws.com`,
            LEMLEARN_ASSETS_BUCKET: props.assetsBucket.bucketName,
        };
        // Groupes de logs déclarés explicitement : `logRetention` crée une Lambda
        // annexe pour poser la rétention après coup, et est déprécié.
        const apiLogs = new logs.LogGroup(this, "ApiLogs", {
            logGroupName: `/aws/lambda/lemlearn-api-${props.envName}`,
            retention: logs.RetentionDays.ONE_YEAR,
        });
        const exportLogs = new logs.LogGroup(this, "ExportLogs", {
            logGroupName: `/aws/lambda/lemlearn-export-${props.envName}`,
            retention: logs.RetentionDays.ONE_YEAR,
        });
        const apiFn = new lambda.Function(this, "ApiFunction", {
            functionName: `lemlearn-api-${props.envName}`,
            runtime: lambda.Runtime.PROVIDED_AL2023,
            architecture: lambda.Architecture.ARM_64,
            handler: "bootstrap",
            code: lambda.Code.fromAsset(path.join(apiRoot, "dist", "lambda")),
            layers: [typstLayer],
            memorySize: 1024,
            timeout: aws_cdk_lib_1.Duration.seconds(30),
            logGroup: apiLogs,
            environment: sharedEnv,
        });
        // L'export d'un dossier assemble des dizaines de pièces et peut dépasser
        // largement le budget d'une requête HTTP : fonction dédiée, appelée en
        // asynchrone, avec un disque temporaire plus large.
        const exportFn = new lambda.Function(this, "ExportFunction", {
            functionName: `lemlearn-export-${props.envName}`,
            runtime: lambda.Runtime.PROVIDED_AL2023,
            architecture: lambda.Architecture.ARM_64,
            handler: "bootstrap",
            code: lambda.Code.fromAsset(path.join(apiRoot, "dist", "lambda")),
            layers: [typstLayer],
            memorySize: 2048,
            timeout: aws_cdk_lib_1.Duration.minutes(5),
            // Un dossier complet — pièces d'identité, PDF scellés, relevés — est
            // assemblé sur disque avant d'être téléversé : 512 Mo n'y suffisent pas.
            ephemeralStorageSize: aws_cdk_lib_1.Size.mebibytes(2048),
            logGroup: exportLogs,
            environment: { ...sharedEnv, LEMLEARN_ROLE: "export" },
        });
        props.table.grantReadWriteData(apiFn);
        props.table.grantReadData(exportFn);
        props.documentsBucket.grantReadWrite(apiFn);
        props.documentsBucket.grantRead(exportFn);
        props.identityBucket.grantReadWrite(apiFn);
        props.identityBucket.grantRead(exportFn);
        props.videoBucket.grantReadWrite(apiFn);
        // Les logos des organismes, et eux seuls. Le préfixe est explicite : le
        // compartiment est ouvert en lecture publique sur `brand/`, et une
        // autorisation d'écriture plus large y ferait entrer n'importe quoi
        // d'autre sous une adresse que tout le monde peut lire.
        props.assetsBucket.grantPut(apiFn, "brand/*");
        props.assetsBucket.grantDelete(apiFn, "brand/*");
        props.assetsBucket.grantPut(apiFn, "covers/*");
        props.assetsBucket.grantDelete(apiFn, "covers/*");
        // Lecture des logos : ils sont incorporés aux PDF plutôt que référencés,
        // pour qu'un document scellé ne dépende pas d'une image qu'un lecteur
        // irait chercher sur le réseau des années plus tard.
        props.assetsBucket.grantRead(apiFn, "brand/*");
        props.assetsBucket.grantRead(exportFn, "brand/*");
        // ---------------------------------------------------------------------
        // Journal d'audit : ajout seul, au niveau IAM
        // ---------------------------------------------------------------------
        // Le chaînage par hash rend une altération *détectable* ; cette politique
        // la rend *impossible* depuis le service. Les deux sont nécessaires :
        // l'une prouve, l'autre empêche.
        const auditAppendOnly = new iam.PolicyStatement({
            effect: iam.Effect.ALLOW,
            // PutItem seulement, et surtout pas BatchWriteItem : la même opération
            // sert à écrire et à supprimer, et IAM ne permet pas de distinguer les
            // deux. Le journal s'écrit événement par événement, dans la même
            // TransactWriteItems que la mutation métier qu'il décrit.
            actions: ["dynamodb:PutItem", "dynamodb:Query", "dynamodb:GetItem"],
            // L'index est une ressource distincte : sans lui, lire le journal dans
            // l'ordre du temps échouerait alors même que la table est autorisée.
            resources: [props.auditTable.tableArn, `${props.auditTable.tableArn}/index/*`],
        });
        const auditDenyMutation = new iam.PolicyStatement({
            // Un Deny explicite l'emporte sur tout Allow, y compris ceux qu'un
            // futur grant*() ajouterait par inadvertance.
            effect: iam.Effect.DENY,
            actions: [
                "dynamodb:UpdateItem",
                "dynamodb:DeleteItem",
                "dynamodb:BatchWriteItem",
                "dynamodb:DeleteTable",
            ],
            resources: [props.auditTable.tableArn, `${props.auditTable.tableArn}/index/*`],
        });
        for (const fn of [apiFn, exportFn]) {
            fn.addToRolePolicy(auditAppendOnly);
            fn.addToRolePolicy(auditDenyMutation);
        }
        exportFn.grantInvoke(apiFn);
        apiFn.addEnvironment("LEMLEARN_EXPORT_FUNCTION", exportFn.functionName);
        // La diffusion n'est branchée que si la pile de données l'a provisionnée.
        if (props.videoDomain && props.videoKeyPairID) {
            apiFn.addEnvironment("LEMLEARN_CDN_DOMAIN", `https://${props.videoDomain}`);
            apiFn.addEnvironment("LEMLEARN_CDN_KEY_ID", props.videoKeyPairID);
            if (props.cloudFrontPrivateKey) {
                apiFn.addEnvironment("LEMLEARN_CDN_KEY", props.cloudFrontPrivateKey);
            }
        }
        // ---------------------------------------------------------------------
        // Transcodage
        // ---------------------------------------------------------------------
        // Le rôle est endossé par MediaConvert, pas par notre Lambda : c'est lui
        // qui lit la source et écrit les rendus, et les droits doivent suivre le
        // service qui agit.
        const mediaConvertRole = new iam.Role(this, "MediaConvertRole", {
            roleName: `Lemlearn-mediaconvert-${props.envName}`,
            assumedBy: new iam.ServicePrincipal("mediaconvert.amazonaws.com"),
            description: "lemlearn: lecture des sources et ecriture des rendus HLS",
        });
        props.videoBucket.grantReadWrite(mediaConvertRole);
        apiFn.addEnvironment("LEMLEARN_MEDIACONVERT_ROLE", mediaConvertRole.roleArn);
        if (props.mediaConvertEndpoint) {
            apiFn.addEnvironment("LEMLEARN_MEDIACONVERT_ENDPOINT", props.mediaConvertEndpoint);
        }
        // Créer et surveiller un travail de transcodage, et confier le rôle
        // ci-dessus au service qui l'exécute.
        apiFn.addToRolePolicy(new iam.PolicyStatement({
            actions: ["mediaconvert:CreateJob", "mediaconvert:GetJob", "mediaconvert:DescribeEndpoints"],
            resources: ["*"],
        }));
        apiFn.addToRolePolicy(new iam.PolicyStatement({
            actions: ["iam:PassRole"],
            resources: [mediaConvertRole.roleArn],
            conditions: { StringEquals: { "iam:PassedToService": "mediaconvert.amazonaws.com" } },
        }));
        // ---------------------------------------------------------------------
        // Satisfaction à froid
        // ---------------------------------------------------------------------
        // Qualiopi demande une mesure de satisfaction à trois mois. C'est
        // l'indicateur que les organismes oublient le plus, parce qu'il tombe
        // longtemps après que tout le monde est passé à autre chose : une règle
        // quotidienne qui relit les échéances du jour est la seule façon qu'il
        // parte sans que personne n'y pense.
        //
        // La même fonction sert l'API et ce travail — elle distingue les deux sur
        // la forme de l'événement. Un artefact séparé aurait les mêmes
        // dépendances et un déploiement de plus à garder en phase.
        new events.Rule(this, "ColdSurveyRule", {
            ruleName: `lemlearn-satisfaction-froid-${props.envName}`,
            description: "lemlearn: relance quotidienne des questionnaires a froid",
            // 7 h UTC : le courriel arrive en début de matinée en France, heure à
            // laquelle un questionnaire de deux minutes a le plus de chances d'être
            // ouvert.
            schedule: events.Schedule.cron({ minute: "0", hour: "7" }),
            targets: [
                new targets.LambdaFunction(apiFn, {
                    event: events.RuleTargetInput.fromObject({ task: "satisfaction-froid" }),
                    // Une relance manquée est rattrapée le lendemain par la requête sur
                    // le mois courant : inutile de réessayer en boucle le jour même.
                    retryAttempts: 2,
                }),
            ],
        });
        // ---------------------------------------------------------------------
        // API HTTP
        // ---------------------------------------------------------------------
        this.api = new apigw.HttpApi(this, "HttpApi", {
            apiName: `lemlearn-${props.envName}`,
            defaultIntegration: new integrations.HttpLambdaIntegration("ApiIntegration", apiFn),
            corsPreflight: {
                allowOrigins: [props.appUrl],
                allowMethods: [
                    apigw.CorsHttpMethod.GET,
                    apigw.CorsHttpMethod.POST,
                    apigw.CorsHttpMethod.PATCH,
                    apigw.CorsHttpMethod.DELETE,
                    apigw.CorsHttpMethod.OPTIONS,
                ],
                allowHeaders: ["Content-Type", "Authorization", "X-Requested-With"],
                // Les sessions passent par un cookie httpOnly, pas par un jeton en
                // en-tête : sans cette ligne, le navigateur ne l'enverrait pas.
                allowCredentials: true,
                maxAge: aws_cdk_lib_1.Duration.hours(1),
            },
        });
        new aws_cdk_lib_1.CfnOutput(this, "ApiUrl", { value: this.api.apiEndpoint });
    }
}
exports.ComputeStack = ComputeStack;
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJmaWxlIjoiY29tcHV0ZS1zdGFjay5qcyIsInNvdXJjZVJvb3QiOiIiLCJzb3VyY2VzIjpbImNvbXB1dGUtc3RhY2sudHMiXSwibmFtZXMiOltdLCJtYXBwaW5ncyI6Ijs7O0FBQUEsNkNBQWdGO0FBQ2hGLHNEQUFzRDtBQUN0RCwwRUFBMEU7QUFFMUUsMkNBQTJDO0FBQzNDLGlEQUFpRDtBQUNqRCxpREFBaUQ7QUFDakQsMERBQTBEO0FBQzFELDZDQUE2QztBQUc3QyxrQ0FBa0M7QUFvQ2xDOzs7Ozs7R0FNRztBQUNILE1BQWEsWUFBYSxTQUFRLG1CQUFLO0lBQzVCLEdBQUcsQ0FBZ0I7SUFFNUIsWUFBWSxLQUFnQixFQUFFLEVBQVUsRUFBRSxLQUF3QjtRQUNoRSxLQUFLLENBQUMsS0FBSyxFQUFFLEVBQUUsRUFBRSxLQUFLLENBQUMsQ0FBQztRQUV4QixNQUFNLE9BQU8sR0FBRyxJQUFJLENBQUMsSUFBSSxDQUFDLFNBQVMsRUFBRSxJQUFJLEVBQUUsSUFBSSxFQUFFLFVBQVUsRUFBRSxLQUFLLENBQUMsQ0FBQztRQUVwRSxvRUFBb0U7UUFDcEUsaUVBQWlFO1FBQ2pFLDhEQUE4RDtRQUM5RCxNQUFNLFVBQVUsR0FBRyxJQUFJLE1BQU0sQ0FBQyxZQUFZLENBQUMsSUFBSSxFQUFFLFlBQVksRUFBRTtZQUM3RCxJQUFJLEVBQUUsTUFBTSxDQUFDLElBQUksQ0FBQyxTQUFTLENBQUMsSUFBSSxDQUFDLElBQUksQ0FBQyxPQUFPLEVBQUUsTUFBTSxFQUFFLE9BQU8sQ0FBQyxDQUFDO1lBQ2hFLGtCQUFrQixFQUFFLENBQUMsTUFBTSxDQUFDLE9BQU8sQ0FBQyxlQUFlLENBQUM7WUFDcEQsdUJBQXVCLEVBQUUsQ0FBQyxNQUFNLENBQUMsWUFBWSxDQUFDLE1BQU0sQ0FBQztZQUNyRCxXQUFXLEVBQUUsMENBQTBDO1NBQ3hELENBQUMsQ0FBQztRQUVILE1BQU0sU0FBUyxHQUEyQjtZQUN4QyxZQUFZLEVBQUUsS0FBSyxDQUFDLE9BQU87WUFDM0IsY0FBYyxFQUFFLEtBQUssQ0FB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