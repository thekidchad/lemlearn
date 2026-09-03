"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.DataStack = void 0;
const aws_cdk_lib_1 = require("aws-cdk-lib");
const dynamodb = require("aws-cdk-lib/aws-dynamodb");
const iam = require("aws-cdk-lib/aws-iam");
const kms = require("aws-cdk-lib/aws-kms");
const cloudfront = require("aws-cdk-lib/aws-cloudfront");
const origins = require("aws-cdk-lib/aws-cloudfront-origins");
const s3 = require("aws-cdk-lib/aws-s3");
/**
 * Socle de données : deux tables DynamoDB et trois compartiments S3.
 *
 * La séparation en une pile dédiée est délibérée : le calcul se redéploie
 * plusieurs fois par jour, les données jamais. Une erreur dans la pile
 * applicative ne doit pas pouvoir toucher un compartiment sous Object Lock.
 */
class DataStack extends aws_cdk_lib_1.Stack {
    table;
    auditTable;
    documentsBucket;
    identityBucket;
    videoBucket;
    assetsBucket;
    identityKey;
    /** Renseignés si la diffusion vidéo est provisionnée. */
    videoDomain;
    videoKeyPairID;
    constructor(scope, id, props) {
        super(scope, id, props);
        const removalPolicy = props.retain ? aws_cdk_lib_1.RemovalPolicy.RETAIN : aws_cdk_lib_1.RemovalPolicy.DESTROY;
        // ---------------------------------------------------------------------
        // Table principale — single-table, isolation tenant par préfixe ORG#
        // ---------------------------------------------------------------------
        this.table = new dynamodb.TableV2(this, "Table", {
            tableName: `lemlearn-${props.envName}`,
            partitionKey: { name: "PK", type: dynamodb.AttributeType.STRING },
            sortKey: { name: "SK", type: dynamodb.AttributeType.STRING },
            billing: dynamodb.Billing.onDemand(),
            // Les heartbeats vidéo bruts et les jetons de session expirent seuls.
            timeToLiveAttribute: "expiresAt",
            pointInTimeRecoverySpecification: { pointInTimeRecoveryEnabled: true },
            // Alimente Firehose → S3 parquet → Athena pour le reporting, afin de
            // ne jamais avoir à scanner la table en production.
            dynamoStream: dynamodb.StreamViewType.NEW_AND_OLD_IMAGES,
            encryption: dynamodb.TableEncryptionV2.awsManagedKey(),
            removalPolicy,
            globalSecondaryIndexes: [
                {
                    // Accès par type d'entité : contacts d'une org, dossiers par étape,
                    // sessions par date, recherche d'un utilisateur par e-mail.
                    indexName: "GSI1",
                    partitionKey: { name: "GSI1PK", type: dynamodb.AttributeType.STRING },
                    sortKey: { name: "GSI1SK", type: dynamodb.AttributeType.STRING },
                },
                {
                    // Autocomplétion : préfixe de nom ou d'e-mail normalisé.
                    indexName: "GSI2",
                    partitionKey: { name: "GSI2PK", type: dynamodb.AttributeType.STRING },
                    sortKey: { name: "GSI2SK", type: dynamodb.AttributeType.STRING },
                    projectionType: dynamodb.ProjectionType.INCLUDE,
                    nonKeyAttributes: ["displayName", "email", "kind"],
                },
            ],
        });
        // ---------------------------------------------------------------------
        // Table d'audit — ajout seul, chaînée par hash
        // ---------------------------------------------------------------------
        // Elle est séparée pour une seule raison : sa politique IAM interdit
        // UpdateItem et DeleteItem (voir ComputeStack). Mélangée à la table
        // principale, cette interdiction serait impossible à exprimer.
        this.auditTable = new dynamodb.TableV2(this, "AuditTable", {
            tableName: `lemlearn-audit-${props.envName}`,
            partitionKey: { name: "PK", type: dynamodb.AttributeType.STRING },
            sortKey: { name: "SK", type: dynamodb.AttributeType.STRING },
            billing: dynamodb.Billing.onDemand(),
            pointInTimeRecoverySpecification: { pointInTimeRecoveryEnabled: true },
            encryption: dynamodb.TableEncryptionV2.awsManagedKey(),
            // Jamais DESTROY, même hors production : perdre un journal d'audit
            // pour libérer un environnement de test est une mauvaise habitude.
            removalPolicy: aws_cdk_lib_1.RemovalPolicy.RETAIN,
            globalSecondaryIndexes: [
                {
                    // Le journal se lit aussi dans l'ordre du temps, et pas seulement
                    // sujet par sujet : « qu'est-il arrivé aujourd'hui » est la question
                    // qu'on pose en premier quand quelque chose cloche.
                    //
                    // La partition est le jour et non le mois : toutes les écritures
                    // d'audit du produit passent par la clé courante, et un mois entier
                    // sur une seule partition concentrerait la charge d'écriture au même
                    // endroit trente fois plus longtemps.
                    indexName: "GSI1",
                    partitionKey: { name: "GSI1PK", type: dynamodb.AttributeType.STRING },
                    sortKey: { name: "GSI1SK", type: dynamodb.AttributeType.STRING },
                },
            ],
        });
        // ---------------------------------------------------------------------
        // Documents scellés — WORM
        // ---------------------------------------------------------------------
        this.documentsBucket = new s3.Bucket(this, "DocumentsBucket", {
            bucketName: `lemlearn-documents-${props.envName}-${this.account}`,
            encryption: s3.BucketEncryption.S3_MANAGED,
            blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
            enforceSSL: true,
            versioned: true,
            objectLockEnabled: true,
            // Dix ans : durée de conservation attendue d'une convention lors d'un
            // contrôle OPCO. En mode COMPLIANCE, ni un administrateur ni le compte
            // racine ne peut supprimer avant l'échéance.
            objectLockDefaultRetention: props.retain
                ? s3.ObjectLockRetention.compliance(aws_cdk_lib_1.Duration.days(3650))
                : s3.ObjectLockRetention.governance(aws_cdk_lib_1.Duration.days(1)),
            removalPolicy: aws_cdk_lib_1.RemovalPolicy.RETAIN,
        });
        // ---------------------------------------------------------------------
        // Ressources publiques — le logo des courriels, et rien d'autre
        // ---------------------------------------------------------------------
        // Un compartiment à part, ouvert en lecture sur le seul préfixe `brand/`.
        // Les trois autres restent fermés : l'un est sous Object Lock, l'autre
        // chiffré par une clé dédiée, le troisième sert des vidéos derrière des
        // URL signées. Ouvrir l'un d'eux pour une image de vingt kilo-octets
        // reviendrait à percer une porte dans un mur qui protège autre chose.
        this.assetsBucket = new s3.Bucket(this, "AssetsBucket", {
            bucketName: `lemlearn-public-${props.envName}-${this.account}`,
            encryption: s3.BucketEncryption.S3_MANAGED,
            // BLOCK_ACLS et non BLOCK_ALL : la politique ci-dessous doit pouvoir
            // s'appliquer. Les ACL, elles, restent interdites — c'est par elles que
            // se produisent les fuites de compartiment.
            blockPublicAccess: s3.BlockPublicAccess.BLOCK_ACLS,
            enforceSSL: true,
            removalPolicy,
            autoDeleteObjects: !props.retain,
            // Le logo monte depuis le navigateur, par URL signée : sans ces règles,
            // le dépôt échoue sur une erreur que le navigateur n'attribue qu'à S3,
            // sans nommer la règle manquante.
            cors: [
                {
                    allowedOrigins: props.appOrigins,
                    allowedMethods: [s3.HttpMethods.PUT],
                    allowedHeaders: ["*"],
                    maxAge: 3000,
                },
            ],
        });
        this.assetsBucket.addToResourcePolicy(new iam.PolicyStatement({
            effect: iam.Effect.ALLOW,
            principals: [new iam.AnyPrincipal()],
            actions: ["s3:GetObject"],
            // Les préfixes sont nommés un par un : le jour où quelqu'un déposera
            // autre chose dans ce compartiment, ce ne sera pas public par accident.
            // `brand/` porte les logos des organismes, `covers/` les visuels des
            // formations — publics tous deux, mais pour des raisons distinctes.
            resources: [
                this.assetsBucket.arnForObjects("brand/*"),
                this.assetsBucket.arnForObjects("covers/*"),
            ],
        }));
        new aws_cdk_lib_1.CfnOutput(this, "AssetsUrl", {
            value: `https://${this.assetsBucket.bucketName}.s3.${this.region}.amazonaws.com`,
        });
        // ---------------------------------------------------------------------
        // Pièces d'identité — clé dédiée, conservation courte
        // ---------------------------------------------------------------------
        this.identityKey = new kms.Key(this, "IdentityKey", {
            description: "lemlearn — chiffrement des pièces d'identité des apprenants",
            enableKeyRotation: true,
            removalPolicy,
        });
        this.identityBucket = new s3.Bucket(this, "IdentityBucket", {
            bucketName: `lemlearn-identity-${props.envName}-${this.account}`,
            encryption: s3.BucketEncryption.KMS,
            encryptionKey: this.identityKey,
            bucketKeyEnabled: true,
            blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
            enforceSSL: true,
            cors: [
                {
                    // Dépôt direct depuis le navigateur, par URL présignée. Les origines
                    // sont nommées plutôt qu'ouvertes : la signature reste
                    // l'autorisation, mais un compartiment qui contient des cartes
                    // d'identité n'a pas à répondre à n'importe quelle page.
                    allowedMethods: [s3.HttpMethods.PUT, s3.HttpMethods.GET, s3.HttpMethods.HEAD],
                    allowedOrigins: props.appOrigins,
                    allowedHeaders: ["*"],
                    maxAge: 3000,
                },
            ],
            lifecycleRules: [
                {
                    // Recommandation CNIL : une pièce d'identité ne se conserve que le
                    // temps de vérifier le dossier, pas la durée de l'archivage légal.
                    id: "suppression-apres-verification",
                    expiration: aws_cdk_lib_1.Duration.days(90),
                },
            ],
            removalPolicy,
            autoDeleteObjects: !props.retain,
        });
        // ---------------------------------------------------------------------
        // Vidéos — source et rendus HLS
        // ---------------------------------------------------------------------
        this.videoBucket = new s3.Bucket(this, "VideoBucket", {
            bucketName: `lemlearn-video-${props.envName}-${this.account}`,
            encryption: s3.BucketEncryption.S3_MANAGED,
            blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
            enforceSSL: true,
            cors: [
                {
                    // Téléversement direct par le navigateur via URL présignée : sans
                    // cela, la vidéo transiterait par la Lambda, plafonnée en taille.
                    // GET et HEAD servent la lecture directe en développement, quand la
                    // distribution n'est pas provisionnée.
                    allowedMethods: [
                        s3.HttpMethods.PUT,
                        s3.HttpMethods.POST,
                        s3.HttpMethods.GET,
                        s3.HttpMethods.HEAD,
                    ],
                    allowedOrigins: ["*"],
                    allowedHeaders: ["*"],
                    maxAge: 3000,
                },
            ],
            lifecycleRules: [
                {
                    id: "sources-en-archivage",
                    // Le master n'est plus lu une fois le HLS produit : il ne sert qu'à
                    // un ré-encodage éventuel.
                    prefix: "sources/",
                    transitions: [
                        {
                            storageClass: s3.StorageClass.INFREQUENT_ACCESS,
                            transitionAfter: aws_cdk_lib_1.Duration.days(30),
                        },
                        {
                            storageClass: s3.StorageClass.GLACIER_INSTANT_RETRIEVAL,
                            transitionAfter: aws_cdk_lib_1.Duration.days(180),
                        },
                    ],
                },
            ],
            removalPolicy,
            autoDeleteObjects: !props.retain,
        });
        // ---------------------------------------------------------------------
        // Diffusion vidéo
        // ---------------------------------------------------------------------
        // La distribution vit avec le compartiment qu'elle sert : l'accès par
        // identité d'origine pose une politique sur ce compartiment, et la placer
        // ailleurs créerait un cycle entre les piles.
        //
        // Les rendus HLS ne sont jamais publics. Chaque lecture exige une URL
        // signée à durée courte : la protection ne vise pas l'enregistrement
        // d'écran, impossible à empêcher, mais le partage de lien.
        if (props.cloudFrontPublicKey) {
            const publicKey = new cloudfront.PublicKey(this, "VideoPublicKey", {
                encodedKey: props.cloudFrontPublicKey,
                comment: "lemlearn: signature des URL de lecture",
            });
            const keyGroup = new cloudfront.KeyGroup(this, "VideoKeyGroup", {
                items: [publicKey],
            });
            const distribution = new cloudfront.Distribution(this, "VideoDistribution", {
                comment: `lemlearn ${props.envName}: modules video`,
                defaultBehavior: {
                    origin: origins.S3BucketOrigin.withOriginAccessControl(this.videoBucket),
                    viewerProtocolPolicy: cloudfront.ViewerProtocolPolicy.REDIRECT_TO_HTTPS,
                    // Le manifeste et les segments ne portent aucune donnée
                    // personnelle : ils se mettent en cache. C'est l'URL signée qui
                    // porte l'autorisation, pas le contenu.
                    cachePolicy: cloudfront.CachePolicy.CACHING_OPTIMIZED,
                    // Le lecteur charge les segments en XHR : sans en-tête CORS, le
                    // navigateur les refuse alors même que CloudFront les a servis, et
                    // la vidéo reste noire sans message utile. La signature reste la
                    // seule autorisation — ouvrir l'en-tête n'ouvre pas le contenu.
                    responseHeadersPolicy: cloudfront.ResponseHeadersPolicy.CORS_ALLOW_ALL_ORIGINS,
                    trustedKeyGroups: [keyGroup],
                },
                // L'Europe et l'Amérique du Nord suffisent : la classe la plus large
                // triple le coût pour des apprenants qui sont en France.
                priceClass: cloudfront.PriceClass.PRICE_CLASS_100,
            });
            this.videoDomain = distribution.distributionDomainName;
            this.videoKeyPairID = publicKey.publicKeyId;
            new aws_cdk_lib_1.CfnOutput(this, "VideoDomain", { value: distribution.distributionDomainName });
        }
    }
}
exports.DataStack = DataStack;
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJmaWxlIjoiZGF0YS1zdGFjay5qcyIsInNvdXJjZVJvb3QiOiIiLCJzb3VyY2VzIjpbImRhdGEtc3RhY2sudHMiXSwibmFtZXMiOltdLCJtYXBwaW5ncyI6Ijs7O0FBQUEsNkNBTXFCO0FBQ3JCLHFEQUFxRDtBQUNyRCwyQ0FBMkM7QUFDM0MsMkNBQTJDO0FBQzNDLHlEQUF5RDtBQUN6RCw4REFBOEQ7QUFDOUQseUNBQXlDO0FBcUJ6Qzs7Ozs7O0dBTUc7QUFDSCxNQUFhLFNBQVUsU0FBUSxtQkFBSztJQUN6QixLQUFLLENBQW1CO0lBQ3hCLFVBQVUsQ0FBbUI7SUFDN0IsZUFBZSxDQUFZO0lBQzNCLGNBQWMsQ0FBWTtJQUMxQixXQUFXLENBQVk7SUFDdkIsWUFBWSxDQUFZO0lBQ3hCLFdBQVcsQ0FBVTtJQUM5Qix5REFBeUQ7SUFDaEQsV0FBVyxDQUFVO0lBQ3JCLGNBQWMsQ0FBVTtJQUVqQyxZQUFZLEtBQWdCLEVBQUUsRUFBVSxFQUFFLEtBQXFCO1FBQzdELEtBQUssQ0FBQyxLQUFLLEVBQUUsRUFBRSxFQUFFLEtBQUssQ0FBQyxDQUFDO1FBRXhCLE1BQU0sYUFBYSxHQUFHLEtBQUssQ0FBQyxNQUFNLENBQUMsQ0FBQyxDQUFDLDJCQUFhLENBQUMsTUFBTSxDQUFDLENBQUMsQ0FBQywyQkFBYSxDQUFDLE9BQU8sQ0FBQztRQUVsRix3RUFBd0U7UUFDeEUscUVBQXFFO1FBQ3JFLHdFQUF3RTtRQUN4RSxJQUFJLENBQUMsS0FBSyxHQUFHLElBQUksUUFBUSxDQUFDLE9BQU8sQ0FBQyxJQUFJLEVBQUUsT0FBTyxFQUFFO1lBQy9DLFNBQVMsRUFBRSxZQUFZLEtBQUssQ0FBQyxPQUFPLEVBQUU7WUFDdEMsWUFBWSxFQUFFLEVBQUUsSUFBSSxFQUFFLElBQUksRUFBRSxJQUFJLEVBQUUsUUFBUSxDQUFDLGFBQWEsQ0FBQyxNQUFNLEVBQUU7WUFDakUsT0FBTyxFQUFFLEVBQUUsSUFBSSxFQUFFLElBQUksRUFBRSxJQUFJLEVBQUUsUUFBUSxDQUFDLGFBQWEsQ0FBQyxNQUFNLEVB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