import { Stack, type StackProps } from "aws-cdk-lib";
import type { Construct } from "constructs";
/** En-tête que la distribution ajoute et que le serveur de rendu exige. */
export declare const EdgeHeader = "x-lemlearn-edge";
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
export declare class WebStack extends Stack {
    readonly distributionDomain: string;
    constructor(scope: Construct, id: string, props: WebStackProps);
}
