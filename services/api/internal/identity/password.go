package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Paramètres Argon2id, alignés sur la recommandation OWASP (m=19 Mio, t=2,
// p=1). Ils tiennent dans le budget mémoire d'une Lambda à 1 Gio tout en
// restant coûteux pour une attaque par dictionnaire.
//
// Ils sont inscrits dans l'empreinte produite : augmenter le coût plus tard
// n'invalide pas les mots de passe existants, qui continuent d'être vérifiés
// avec leurs propres paramètres.
const (
	argonMemory  uint32 = 19456
	argonTime    uint32 = 2
	argonThreads uint8  = 1
	argonKeyLen  uint32 = 32
	argonSaltLen        = 16
)

// HashPassword produit une empreinte au format PHC.
func HashPassword(password string) (string, error) {
	if len(password) < 12 {
		// Douze caractères plutôt qu'un jeu de règles de composition : la
		// longueur est le seul facteur qui compte réellement, et les règles
		// « une majuscule, un chiffre » produisent des mots de passe plus
		// prévisibles, pas plus solides.
		return "", fmt.Errorf("le mot de passe doit faire au moins 12 caractères")
	}

	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("sel aléatoire: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword contrôle un mot de passe contre son empreinte.
//
// La comparaison est à temps constant : une comparaison ordinaire laisserait
// fuir la longueur du préfixe correct par le temps de réponse.
func VerifyPassword(password, encoded string) bool {
	memory, time, threads, salt, key, err := decodePHC(encoded)
	if err != nil {
		return false
	}
	candidate := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(key)))
	return subtle.ConstantTimeCompare(candidate, key) == 1
}

func decodePHC(encoded string) (memory, time uint32, threads uint8, salt, key []byte, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return 0, 0, 0, nil, nil, fmt.Errorf("empreinte au format inattendu")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("version illisible: %w", err)
	}
	if version != argon2.Version {
		return 0, 0, 0, nil, nil, fmt.Errorf("version argon2 %d non gérée", version)
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("paramètres illisibles: %w", err)
	}
	if salt, err = base64.RawStdEncoding.DecodeString(parts[4]); err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("sel illisible: %w", err)
	}
	if key, err = base64.RawStdEncoding.DecodeString(parts[5]); err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("empreinte illisible: %w", err)
	}
	return memory, time, threads, salt, key, nil
}

// NewToken produit un jeton opaque de 256 bits et son empreinte.
//
// Le jeton part au navigateur ou dans un lien de signature ; seule l'empreinte
// est conservée. Le renvoyer en deux valeurs distinctes rend impossible
// d'écrire par mégarde le jeton en clair dans la base.
func NewToken() (token, hash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("jeton aléatoire: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, HashToken(token), nil
}

// HashToken renvoie l'empreinte d'un jeton.
//
// SHA-256 nu, sans étirement : un jeton de 256 bits tiré aléatoirement n'est
// pas attaquable par dictionnaire, contrairement à un mot de passe. Y appliquer
// Argon2 coûterait 40 ms à chaque requête authentifiée pour rien.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// NewOTP produit un code numérique à six chiffres pour la signature.
func NewOTP() (string, error) {
	max := 1000000
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("code aléatoire: %w", err)
	}
	// Tirage sans biais : on rejette les valeurs qui rendraient certains
	// codes plus probables que d'autres.
	value := int(buf[0])<<24 | int(buf[1])<<16 | int(buf[2])<<8 | int(buf[3])
	value &= 0x7fffffff
	limit := (1 << 31) - (1<<31)%max
	for value >= limit {
		if _, err := rand.Read(buf); err != nil {
			return "", fmt.Errorf("code aléatoire: %w", err)
		}
		value = int(buf[0])<<24 | int(buf[1])<<16 | int(buf[2])<<8 | int(buf[3])
		value &= 0x7fffffff
	}
	return fmt.Sprintf("%06d", value%max), nil
}
