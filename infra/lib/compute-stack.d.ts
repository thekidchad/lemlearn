import { Stack, type StackProps } from "aws-cdk-lib";
import * as apigw from "aws-cdk-lib/aws-apigatewayv2";
import type * as dynamodb from "aws-cdk-lib/aws-dynamodb";
import type * as s3 from "aws-cdk-lib/aws-s3";
import type { Construct } from "constructs";
export interface ComputeStackProps extends StackProps {
    readonly envName: string;
    readonly table: dynamodb.TableV2;
    readonly auditTable: dynamodb.TableV2;
    readonly documentsBucket: s3.Bucket;
    readonly identityBucket: s3.Bucket;
    readonly videoBucket: s3.Bucket;
    /** Compartiment des ressources publiques : le logo des courriels. */
    readonly assetsBucket: s3.Bucket;
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
    /**
     * Adresses de l'équipe lemlearn, séparées par des virgules. Le rôle
     * super-admin s'attribue d'après cette liste à la connexion, et se retire
     * de la même façon : il n'est pas modifiable depuis l'application, sans
     * quoi un accès inter-organisations se donnerait tout seul.
     */
    readonly superAdmins?: string;
}
/**
 * Calcul : une Lambda unique sert toute l'API, routée par chi côté Go.
 *
 * Une fonction par endpoint multiplierait les démarrages à froid et les
 * déploiements partiels ; ici un seul artefact, une seule version, un seul
 * rollback.
 */
export declare class ComputeStack extends Stack {
    readonly api: apigw.HttpApi;
    constructor(scope: Construct, id: string, props: ComputeStackProps);
}
