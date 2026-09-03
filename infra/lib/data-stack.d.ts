import { Stack, type StackProps } from "aws-cdk-lib";
import * as dynamodb from "aws-cdk-lib/aws-dynamodb";
import * as kms from "aws-cdk-lib/aws-kms";
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
export declare class DataStack extends Stack {
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
    constructor(scope: Construct, id: string, props: DataStackProps);
}
