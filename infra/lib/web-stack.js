"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.WebStack = exports.EdgeHeader = void 0;
const aws_cdk_lib_1 = require("aws-cdk-lib");
const cloudfront = require("aws-cdk-lib/aws-cloudfront");
const origins = require("aws-cdk-lib/aws-cloudfront-origins");
const lambda = require("aws-cdk-lib/aws-lambda");
const logs = require("aws-cdk-lib/aws-logs");
const path = require("node:path");
/** En-tête que la distribution ajoute et que le serveur de rendu exige. */
exports.EdgeHeader = "x-lemlearn-edge";
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
class WebStack extends aws_cdk_lib_1.Stack {
    distributionDomain;
    constructor(scope, id, props) {
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
            code: lambda.Code.fromAsset(path.join(__dirname, "..", "..", "apps", "web", "dist", "lambda")),
            // Le rendu React est gourmand en processeur, et la part de processeur
            // allouée suit la mémoire : moins de mémoire rendrait chaque page plus
            // lente pour un coût identique à la milliseconde près.
            memorySize: 1769,
            timeout: aws_cdk_lib_1.Duration.seconds(30),
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
            customHeaders: { [exports.EdgeHeader]: props.edgeSecret },
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
        new aws_cdk_lib_1.CfnOutput(this, "WebUrl", { value: `https://${distribution.distributionDomainName}` });
    }
}
exports.WebStack = WebStack;
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJmaWxlIjoid2ViLXN0YWNrLmpzIiwic291cmNlUm9vdCI6IiIsInNvdXJjZXMiOlsid2ViLXN0YWNrLnRzIl0sIm5hbWVzIjpbXSwibWFwcGluZ3MiOiI7OztBQUFBLDZDQUEwRTtBQUMxRSx5REFBeUQ7QUFDekQsOERBQThEO0FBQzlELGlEQUFpRDtBQUNqRCw2Q0FBNkM7QUFFN0Msa0NBQWtDO0FBRWxDLDJFQUEyRTtBQUM5RCxRQUFBLFVBQVUsR0FBRyxpQkFBaUIsQ0FBQztBQTBCNUM7Ozs7Ozs7Ozs7OztHQVlHO0FBQ0gsTUFBYSxRQUFTLFNBQVEsbUJBQUs7SUFDeEIsa0JBQWtCLENBQVM7SUFFcEMsWUFBWSxLQUFnQixFQUFFLEVBQVUsRUFBRSxLQUFvQjtRQUM1RCxLQUFLLENBQUMsS0FBSyxFQUFFLEVBQUUsRUFBRSxLQUFLLENBQUMsQ0FBQztRQUV4QixNQUFNLFFBQVEsR0FBRyxJQUFJLElBQUksQ0FBQyxRQUFRLENBQUMsSUFBSSxFQUFFLFNBQVMsRUFBRTtZQUNsRCxZQUFZLEVBQUUsNEJBQTRCLEtBQUssQ0FBQyxPQUFPLEVBQUU7WUFDekQsU0FBUyxFQUFFLElBQUksQ0FBQyxhQUFhLENBQUMsU0FBUztTQUN4QyxDQUFDLENBQUM7UUFFSCxNQUFNLFFBQVEsR0FBRyxJQUFJLE1BQU0sQ0FBQyxRQUFRLENBQUMsSUFBSSxFQUFFLGFBQWEsRUFBRTtZQUN4RCxZQUFZLEVBQUUsZ0JBQWdCLEtBQUssQ0FBQyxPQUFPLEVBQUU7WUFDN0MsT0FBTyxFQUFFLE1BQU0sQ0FBQyxPQUFPLENBQUMsV0FBVztZQUNuQyxZQUFZLEVBQUUsTUFBTSxDQUFDLFlBQVksQ0FBQyxNQUFNO1lBQ3hDLE9BQU8sRUFBRSxpQ0FBaUM7WUFDMUMsSUFBSSxFQUFFLE1BQU0sQ0FBQyxJQUFJLENBQUMsU0FBUyxDQUN6QixJQUFJLENBQUMsSUFBSSxDQUFDLFNBQVMsRUFBRSxJQUFJLEVBQUUsSUFBSSxFQUFFLE1BQU0sRUFBRSxLQUFLLEVBQUUsTUFBTSxFQUFFLFFBQVEsQ0FBQyxD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