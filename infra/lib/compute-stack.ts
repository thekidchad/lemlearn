import { CfnOutput, Duration, Size, Stack, type StackProps } from "aws-cdk-lib";
import * as apigw from "aws-cdk-lib/aws-apigatewayv2";
import * as integrations from "aws-cdk-lib/aws-apigatewayv2-integrations";
import type * as dynamodb from "aws-cdk-lib/aws-dynamodb";
import * as iam from "aws-cdk-lib/aws-iam";
import * as lambda from "aws-cdk-lib/aws-lambda";
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
