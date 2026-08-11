import {
  Duration,
  RemovalPolicy,
  Stack,
  type StackProps,
} from "aws-cdk-lib";
import * as dynamodb from "aws-cdk-lib/aws-dynamodb";
import * as kms from "aws-cdk-lib/aws-kms";
import * as s3 from "aws-cdk-lib/aws-s3";
import type { Construct } from "constructs";

export interface DataStackProps extends StackProps {
  readonly envName: string;
  /** Les environnements de production ne se détruisent pas par mégarde. */
  readonly retain: boolean;
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
  readonly identityKey: kms.Key;

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
          allowedMethods: [s3.HttpMethods.PUT, s3.HttpMethods.POST],
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
  }
}
