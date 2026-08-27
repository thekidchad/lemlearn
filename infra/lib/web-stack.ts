import { CfnOutput, Duration, Stack, type StackProps } from "aws-cdk-lib";
import * as cloudfront from "aws-cdk-lib/aws-cloudfront";
import * as origins from "aws-cdk-lib/aws-cloudfront-origins";
import * as lambda from "aws-cdk-lib/aws-lambda";
import * as logs from "aws-cdk-lib/aws-logs";
import type { Construct } from "constructs";
import * as path from "node:path";

/** En-tête que la distribution ajoute et que le serveur de rendu exige. */
export const EdgeHeader = "x-lemlearn-edge";

export interface WebStackProps extends StackProps {
  readonly envName: string;
  /** Adresse de l'API, injectée dans le serveur de rendu. */
  readonly apiUrl: string;
  /**
   * Nom du cookie de session posé par l'API. Il diffère du nôtre quand les
   * deux ne tournent pas dans le même environnement — et une différence non
   * déclarée renvoie chaque appel authentifié en 401 sans rien expliquer.
   */
  readonly apiCookie?: string;
  /**
   * Secret partagé entre la distribution et le serveur de rendu : il rend
   * l'URL de fonction inutilisable à qui ne passe pas par CloudFront.
   */
  readonly edgeSecret: string;
  /**
   * Adresse publique du front. Le serveur Next est joint sur 127.0.0.1 et
   * croit donc servir cet hôte-là : il refuse alors toute action serveur, dont
   * l'en-tête `Origin` nomme le vrai domaine — c'est sa protection contre les
   * requêtes forgées depuis un autre site. Le rendu la lui rétablit.
   */
  readonly publicUrl?: string;
}

/**
 * Hébergement du front : le serveur Next dans une Lambda, derrière CloudFront.
 *
 * Le rendu est fait par des composants serveur qui lisent le cookie de session
 * — un export statique est donc exclu. Restait à choisir entre un conteneur et
 * une fonction : la fonction évite un registre d'images, une tâche à
 * dimensionner et un coût au repos, pour un produit dont le trafic est celui
 * d'un outil de bureau.
 *
 * Le serveur autonome produit par Next est démarré une fois par conteneur sur
 * la boucle locale, et chaque événement lui est transmis ; c'est
 * `apps/web/lambda-handler.js`.
 */
export class WebStack extends Stack {
  readonly distributionDomain: string;

  constructor(scope: Construct, id: string, props: WebStackProps) {
    super(scope, id, props);

    const logGroup = new logs.LogGroup(this, "WebLogs", {
      logGroupName: `/aws/lambda/lemlearn-web-${props.envName}`,
      retention: logs.RetentionDays.ONE_MONTH,
    });

    const renderer = new lambda.Function(this, "WebFunction", {
      functionName: `lemlearn-web-${props.envName}`,
      runtime: lambda.Runtime.NODEJS_22_X,
      architecture: lambda.Architecture.ARM_64,
      handler: "apps/web/lambda-handler.handler",
      code: lambda.Code.fromAsset(
        path.join(__dirname, "..", "..", "apps", "web", "dist", "lambda"),
      ),
      // Le rendu React est gourmand en processeur, et la part de processeur
      // allouée suit la mémoire : moins de mémoire rendrait chaque page plus
      // lente pour un coût identique à la milliseconde près.
      memorySize: 1769,
      timeout: Duration.seconds(30),
      logGroup,
      environment: {
        LEMLEARN_API_URL: props.apiUrl,
        ...(props.apiCookie ? { LEMLEARN_API_COOKIE: props.apiCookie } : {}),
        NODE_ENV: "production",
        // Le serveur écoute sur la boucle locale du conteneur : rien
        // n'atteint ce port depuis l'extérieur.
        NEXT_PORT: "3000",
        LEMLEARN_EDGE_SECRET: props.edgeSecret,
        ...(props.publicUrl
          ? { LEMLEARN_PUBLIC_HOST: props.publicUrl.replace(/^https?:\/\//, "").replace(/\/.*$/, "") }
          : {}),
      },
    });

    // L'URL de fonction reste joignable par le réseau, mais elle n'est utile à
    // personne : la distribution y ajoute un secret que le serveur de rendu
    // exige, et refuse tout ce qui ne le porte pas. La signature SigV4 par
    // identité d'origine serait plus propre, mais Lambda refuse ici les
    // requêtes signées par CloudFront sans en donner la raison — autorisation
    // de ressource, identité d'origine et politique de transmission conformes
    // à la documentation, et 403 quand même.
    const url = renderer.addFunctionUrl({
      authType: lambda.FunctionUrlAuthType.NONE,
    });

    const origin = new origins.FunctionUrlOrigin(url, {
      customHeaders: { [EdgeHeader]: props.edgeSecret },
    });

    // Tout est transmis au serveur de rendu sauf `Host`, qui doit rester celui
    // de la fonction. Le secret de bordure n'a pas à figurer ici : CloudFront
    // refuse qu'un en-tête soit à la fois ajouté par l'origine et déclaré dans
    // la transmission, parce que la valeur de l'origine écrase de toute façon
    // celle qu'un visiteur aurait envoyée.
    const forwarding = new cloudfront.OriginRequestPolicy(this, "WebForwarding", {
      originRequestPolicyName: `lemlearn-web-${props.envName}`,
      comment: "Transmet la requête au rendu, sans l'en-tête Host",
      cookieBehavior: cloudfront.OriginRequestCookieBehavior.all(),
      queryStringBehavior: cloudfront.OriginRequestQueryStringBehavior.all(),
      headerBehavior: cloudfront.OriginRequestHeaderBehavior.denyList("host"),
    });

    const distribution = new cloudfront.Distribution(this, "WebDistribution", {
      comment: `lemlearn ${props.envName}: application`,
      defaultBehavior: {
        origin,
        viewerProtocolPolicy: cloudfront.ViewerProtocolPolicy.REDIRECT_TO_HTTPS,
        allowedMethods: cloudfront.AllowedMethods.ALLOW_ALL,
        // Rien n'est mis en cache par défaut : chaque page dépend du cookie de
        // session, et une page d'un organisme servie à un autre serait pire
        // qu'une lenteur.
        cachePolicy: cloudfront.CachePolicy.CACHING_DISABLED,
        originRequestPolicy: forwarding,
      },
      additionalBehaviors: {
        // Les fichiers produits par la compilation portent une empreinte dans
        // leur nom : ils ne changent jamais sous un même nom, donc ils se
        // mettent en cache sans réserve.
        "/_next/static/*": {
          origin,
          viewerProtocolPolicy: cloudfront.ViewerProtocolPolicy.REDIRECT_TO_HTTPS,
          cachePolicy: cloudfront.CachePolicy.CACHING_OPTIMIZED,
        },
        "/brand/*": {
          origin,
          viewerProtocolPolicy: cloudfront.ViewerProtocolPolicy.REDIRECT_TO_HTTPS,
          cachePolicy: cloudfront.CachePolicy.CACHING_OPTIMIZED,
        },
        "/photos/*": {
          origin,
          viewerProtocolPolicy: cloudfront.ViewerProtocolPolicy.REDIRECT_TO_HTTPS,
          cachePolicy: cloudfront.CachePolicy.CACHING_OPTIMIZED,
        },
      },
      priceClass: cloudfront.PriceClass.PRICE_CLASS_100,
      httpVersion: cloudfront.HttpVersion.HTTP2_AND_3,
    });

    this.distributionDomain = distribution.distributionDomainName;

    new CfnOutput(this, "WebUrl", { value: `https://${distribution.distributionDomainName}` });
  }
}
