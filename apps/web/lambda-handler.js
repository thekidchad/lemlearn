/**
 * Adaptateur Lambda pour le serveur Next autonome.
 *
 * Next produit un serveur HTTP complet ; Lambda ne parle pas HTTP mais des
 * événements JSON. Plutôt que d'ajouter une couche tierce, on démarre le
 * serveur une fois par conteneur sur la boucle locale et on lui transmet
 * chaque événement — quelques dizaines de lignes, et rien à maintenir de plus
 * que ce que Next produit déjà.
 *
 * Le serveur survit entre deux invocations : c'est ce qui fait qu'un démarrage
 * à froid ne coûte qu'une fois.
 */
const crypto = require("crypto");
const path = require("path");
const zlib = require("zlib");
const { promisify } = require("util");

const gzip = promisify(zlib.gzip);

/**
 * En dessous de ce seuil, l'en-tête de compression coûte plus que ce qu'il
 * fait gagner. C'est le seuil qu'emploie CloudFront pour les mêmes raisons.
 */
const COMPRESS_FROM = 1000;

const PORT = Number(process.env.NEXT_PORT || 3000);
/** Doit rester identique à `EdgeHeader` dans `infra/lib/web-stack.ts`. */
const EDGE_HEADER = "x-lemlearn-edge";
let booting;

function boot() {
  if (booting) return booting;

  // La configuration résolue à la compilation, telle que Next l'a écrite.
  // Sans elle, le serveur relit `next.config.ts` au démarrage, donc réclame
  // SWC pour analyser du TypeScript — et tente de le télécharger depuis npm,
  // ce qu'une Lambda ne peut ni ne doit faire.
  const required = require(path.join(__dirname, ".next", "required-server-files.json"));
  process.env.__NEXT_PRIVATE_STANDALONE_CONFIG = JSON.stringify(required.config);
  process.env.NODE_ENV = "production";

  const { startServer } = require("next/dist/server/lib/start-server");
  booting = startServer({
    dir: __dirname,
    hostname: "127.0.0.1",
    port: PORT,
    isDev: false,
    minimalMode: false,
  });
  return booting;
}

/**
 * Les en-têtes qui peuvent apparaître plusieurs fois ne tiennent pas dans un
 * objet clé-valeur : les URL de fonction attendent les cookies dans un champ
 * à part, sans quoi seul le dernier survivrait — et une session posée en même
 * temps qu'un thème en perdrait une des deux.
 */
function splitCookies(headers) {
  const cookies = headers.getSetCookie ? headers.getSetCookie() : [];
  const plain = {};
  headers.forEach((value, key) => {
    const name = key.toLowerCase();
    if (name === "set-cookie") return;
    // `fetch` a déjà décompressé le corps : ces deux en-têtes décrivent la
    // réponse d'avant. Le premier est retiré, le second recalculé plus bas —
    // CloudFront ne comprime que ce dont il connaît la taille, et sans lui la
    // page part en clair.
    if (name === "content-encoding" || name === "content-length") return;
    plain[key] = value;
  });
  return { cookies, plain };
}

/**
 * L'URL de fonction est joignable par tout le monde ; ce secret est ce qui la
 * réserve à la distribution. On compare en temps constant : une comparaison
 * naïve fuit, caractère par caractère, de quoi deviner le secret.
 */
function fromEdge(headers) {
  const expected = process.env.LEMLEARN_EDGE_SECRET;
  if (!expected) return true;
  const got = headers?.[EDGE_HEADER] ?? "";
  const a = Buffer.from(String(got));
  const b = Buffer.from(expected);
  return a.length === b.length && crypto.timingSafeEqual(a, b);
}

exports.handler = async (event) => {
  if (!fromEdge(event.headers)) {
    return {
      statusCode: 403,
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ error: "origine refusée" }),
    };
  }

  await boot();

  const request = event.requestContext?.http ?? {};
  const query = event.rawQueryString ? `?${event.rawQueryString}` : "";
  const url = `http://127.0.0.1:${PORT}${event.rawPath || "/"}${query}`;

  const headers = { ...event.headers };
  // L'en-tête d'hôte de la passerelle n'a pas de sens pour Next : c'est
  // l'hôte transmis par CloudFront qui décrit l'adresse vue par le visiteur.
  delete headers["content-length"];
  // On ne demande rien de compressé au serveur local : `fetch` décompresserait
  // la réponse sans retirer l'en-tête qui l'annonce, et le navigateur, lui, le
  // croirait — page blanche, sans la moindre erreur au journal. La compression
  // est de toute façon le travail de CloudFront, qui la fait pour le visiteur.
  delete headers["accept-encoding"];
  // Next ne voit que 127.0.0.1 et refuserait chaque action serveur, dont
  // l'en-tête `Origin` nomme le domaine public. On lui rend cet hôte-là.
  if (process.env.LEMLEARN_PUBLIC_HOST) {
    headers["x-forwarded-host"] = process.env.LEMLEARN_PUBLIC_HOST;
    headers["x-forwarded-proto"] = "https";
  }
  if (Array.isArray(event.cookies) && event.cookies.length > 0) {
    headers.cookie = event.cookies.join("; ");
  }

  const body = event.body
    ? event.isBase64Encoded
      ? Buffer.from(event.body, "base64")
      : Buffer.from(event.body)
    : undefined;

  const response = await fetch(url, {
    method: request.method || "GET",
    headers,
    body,
    redirect: "manual",
  });

  const { cookies, plain } = splitCookies(response.headers);
  let payload = Buffer.from(await response.arrayBuffer());
  const type = response.headers.get("content-type") || "";
  // Le texte passe tel quel ; tout le reste — images, polices, PDF — doit
  // être encodé, sinon il arrive corrompu.
  let isText = /^(text\/|application\/(json|javascript|xml)|image\/svg)/.test(type);

  // CloudFront ne comprime que ce qu'il met en cache, et le rendu ne l'est
  // pas : sans cette compression, chaque page partirait en clair — cent
  // cinquante kilooctets pour un tableau de bord qu'on ouvre vingt fois par
  // jour.
  const accepted = event.headers?.["accept-encoding"] ?? "";
  if (isText && payload.length >= COMPRESS_FROM && /\bgzip\b/.test(accepted)) {
    payload = await gzip(payload);
    plain["content-encoding"] = "gzip";
    isText = false;
  }

  if (payload.length > 0) plain["content-length"] = String(payload.length);

  return {
    statusCode: response.status,
    headers: plain,
    cookies,
    isBase64Encoded: !isText,
    body: isText ? payload.toString("utf8") : payload.toString("base64"),
  };
};
