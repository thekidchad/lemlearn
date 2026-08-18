import { CfnOutput, Duration, Size, Stack, type StackProps } from "aws-cdk-lib";
import * as apigw from "aws-cdk-lib/aws-apigatewayv2";
import * as integrations from "aws-cdk-lib/aws-apigatewayv2-integrations";
import type * as dynamodb from "aws-cdk-lib/aws-dynamodb";
import * as iam from "aws-cdk-lib/aws-iam";
import * as lambda from "aws-cdk-lib/aws-lambda";
import * as events from "aws-cdk-lib/aws-events";
import * as targets from "aws-cdk-lib/aws-events-targets";
import * as logs from "aws-cdk-lib/aws-logs";
import type * as s3 from "aws-cdk-lib/aws-s3";
import type { Construct } from "constructs";
import * as path from "node:path";

export interface ComputeStackProps extends StackProps {
  readonly envName: string;
  readonly table: dynamodb.TableV2;
  readonly auditTable: dynamodb.TableV2;
  readonly documentsBucket: s3.Bucket;
  readonly identityBucket: s3.Bucket;
  readonly videoBucket: s3.Bucket;
  readonly appUrl: string;
  /**
   * Domaine et identifiant de clé de la distribution vidéo, produits par la
   * pile de données — c'est elle qui porte le compartiment, donc la
   * distribution qui le sert.
   */
  readonly videoDomain?: string;
  readonly videoKeyPairID?: string;
  /**
   * Clé privée de signature des URL de lecture, posée en variable
   * d'environnement de la fonction. Acceptable en recette ; en production elle
   * a sa place dans Secrets Manager, référencée plutôt que recopiée.
   */
  readonly cloudFrontPrivateKey?: string;
  /** Point d'entrée MediaConvert du compte, propre à chaque compte. */
  readonly mediaConvertEndpoint?: string;
}

/**
 * Calcul : une Lambda unique sert toute l'API, routée par chi côté Go.
 *
 * Une fonction par endpoint multiplierait les démarrages à froid et les
 * déploiements partiels ; ici un seul artefact, une seule version, un seul
 * rollback.
 */
export class ComputeStack extends Stack {
  readonly api: apigw.HttpApi;

  constructor(scope: Construct, id: string, props: ComputeStackProps) {
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

    const sharedEnv: Record<string, string> = {
      LEMLEARN_ENV: props.envName,
      LEMLEARN_TABLE: props.table.tableName,
      LEMLEARN_AUDIT_TABLE: props.auditTable.tableName,
      LEMLEARN_DOCUMENTS_BUCKET: props.documentsBucket.bucketName,
      LEMLEARN_IDENTITY_BUCKET: props.identityBucket.bucketName,
      LEMLEARN_VIDEO_BUCKET: props.videoBucket.bucketName,
      LEMLEARN_APP_URL: props.appUrl,
      TYPST_PATH: "/opt/bin/typst",
      TYPST_FONT_PATH: "/opt/fonts",
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
      timeout: Duration.seconds(30),
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
      timeout: Duration.minutes(5),
      // Un dossier complet — pièces d'identité, PDF scellés, relevés — est
      // assemblé sur disque avant d'être téléversé : 512 Mo n'y suffisent pas.
      ephemeralStorageSize: Size.mebibytes(2048),
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
      resources: [props.auditTable.tableArn],
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
      resources: [props.auditTable.tableArn],
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
        maxAge: Duration.hours(1),
      },
    });

    new CfnOutput(this, "ApiUrl", { value: this.api.apiEndpoint });
  }
}
