import {
  CfnOutput,
  Duration,
  RemovalPolicy,
  Stack,
  type StackProps,
} from "aws-cdk-lib";
import * as dynamodb from "aws-cdk-lib/aws-dynamodb";
import * as iam from "aws-cdk-lib/aws-iam";
import * as kms from "aws-cdk-lib/aws-kms";
import * as cloudfront from "aws-cdk-lib/aws-cloudfront";
import * as origins from "aws-cdk-lib/aws-cloudfront-origins";
import * as s3 from "aws-cdk-lib/aws-s3";
import type { Construct } from "constructs";

export interface DataStackProps extends StackProps {
  readonly envName: string;
  /** Les environnements de production ne se détruisent pas par mégarde. */
  readonly retain: boolean;
  /**
   * Clé publique CloudFront au format PEM. Absente, la diffusion vidéo n'est
   * pas provisionnée : un organisme qui ne fait que du présentiel n'a rien à
   * diffuser, et le reste du produit ne doit pas en dépendre.
   */
  readonly cloudFrontPublicKey?: string;
  /**
   * Origines autorisées à déposer un fichier depuis un navigateur. Les pièces
   * d'identité montent en direct : sans règle CORS, le navigateur refuse la
   * requête avant même que S3 ne la voie.
   */
  readonly appOrigins: string[];
}

/**
 * Socle de données : deux tables DynamoDB et trois compartiments S3.
 *
 * La séparation en une pile dédiée est délibérée : le calcul se redéploie
 * plusieurs fois par jour, les données jamais. Une erreur dans la pile
 * applicative ne doit pas pouvoir toucher un compartiment sous Object Lock.
 */
export class DataStack extends Stack {
  readonly table: dynamodb.TableV2;
  readonly auditTable: dynamodb.TableV2;
  readonly documentsBucket: s3.Bucket;
  readonly identityBucket: s3.Bucket;
  readonly videoBucket: s3.Bucket;
  readonly assetsBucket: s3.Bucket;
  readonly identityKey: kms.Key;
  /** Renseignés si la diffusion vidéo est provisionnée. */
  readonly videoDomain?: string;
  readonly videoKeyPairID?: string;

  constructor(scope: Construct, id: string, props: DataStackProps) {
    super(scope, id, props);

    const removalPolicy = props.retain ? RemovalPolicy.RETAIN : RemovalPolicy.DESTROY;

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
      removalPolicy: RemovalPolicy.RETAIN,
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
        ? s3.ObjectLockRetention.compliance(Duration.days(3650))
        : s3.ObjectLockRetention.governance(Duration.days(1)),
      removalPolicy: RemovalPolicy.RETAIN,
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

    this.assetsBucket.addToResourcePolicy(
      new iam.PolicyStatement({
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
      }),
    );

    new CfnOutput(this, "AssetsUrl", {
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
          expiration: Duration.days(90),
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
              transitionAfter: Duration.days(30),
            },
            {
              storageClass: s3.StorageClass.GLACIER_INSTANT_RETRIEVAL,
              transitionAfter: Duration.days(180),
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
      new CfnOutput(this, "VideoDomain", { value: distribution.distributionDomainName });
    }
  }
}
