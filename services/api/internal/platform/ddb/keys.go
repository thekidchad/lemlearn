// Package ddb est l'accès à DynamoDB en conception mono-table.
//
// Toutes les données d'une organisation partagent la même clé de partition
// `ORG#<id>` : l'isolation entre clients est donc structurelle, pas
// conditionnelle. Un bug de filtre ne peut pas exposer les données d'un autre
// organisme, parce que la requête n'atteint jamais leur partition.
//
// Les clés sont construites ici et nulle part ailleurs : une chaîne « ORG# »
// écrite à la main dans un dépôt est le début d'une divergence.
package ddb

import (
	"fmt"
	"strings"
)

// Préfixes de clé. Ils font partie du schéma : les modifier impose une
// migration de toutes les données existantes.
const (
	prefixOrg     = "ORG#"
	prefixEmail   = "EMAIL#"
	prefixSession = "SESSION#"
	prefixToken   = "TOKEN#"
)

// OrgPK est la clé de partition de tout ce qui appartient à une organisation.
func OrgPK(orgID string) string { return prefixOrg + orgID }

// OrgMetaSK est la clé de tri de la fiche de l'organisation elle-même.
const OrgMetaSK = "META"

func UserSK(userID string) string       { return "USER#" + userID }
func ContactSK(contactID string) string { return "CONTACT#" + contactID }
func FileSK(fileID string) string       { return "FILE#" + fileID }
func CourseSK(courseID string) string   { return "COURSE#" + courseID }
func ModuleSK(courseID, moduleID string) string {
	return "COURSE#" + courseID + "#MOD#" + moduleID
}
func SessionSK(sessionID string) string { return "SESSION#" + sessionID }
func EnrollmentSK(sessionID, contactID string) string {
	return "SESSION#" + sessionID + "#ENR#" + contactID
}
func DocumentSK(documentID string) string { return "DOC#" + documentID }

// WatchSK porte le relevé d'assiduité d'un apprenant sur un module.
func WatchSK(enrollmentID, moduleID string) string {
	return "ENR#" + enrollmentID + "#WATCH#" + moduleID
}
func SignatureSK(requestID string) string { return "SIG#" + requestID }

// --- Éléments globaux (hors partition d'organisation) ---------------------

// EmailPointerPK indexe un e-mail vers son organisation et son utilisateur.
//
// Ce n'est pas un GSI mais un élément à part entière, écrit sous condition
// `attribute_not_exists` : c'est ce qui rend l'unicité d'un e-mail réellement
// garantie. Un index secondaire, éventuellement cohérent, ne peut pas la
// garantir — deux inscriptions simultanées passeraient toutes les deux.
func EmailPointerPK(email string) string { return prefixEmail + NormalizeEmail(email) }

// EmailPointerSK est constant : un e-mail ne pointe que vers un compte.
const EmailPointerSK = "USER"

// AuthSessionPK indexe une session par l'empreinte de son jeton.
//
// Le jeton en clair n'est jamais stocké : une fuite de la base ne permet donc
// pas de rejouer les sessions actives.
func AuthSessionPK(tokenHash string) string { return prefixSession + tokenHash }

// AuthSessionSK est constant.
const AuthSessionSK = "META"

// SignatureTokenPK indexe une demande de signature par l'empreinte du jeton
// contenu dans le lien envoyé au signataire.
func SignatureTokenPK(tokenHash string) string { return prefixToken + tokenHash }

// SignatureTokenSK est constant.
const SignatureTokenSK = "SIG"

// --- Index secondaires ----------------------------------------------------

// GSI1 sert les listes : contacts d'une organisation par type, dossiers par
// étape, sessions par date.
func GSI1Contacts(orgID, kind string) string {
	return OrgPK(orgID) + "#KIND#" + kind
}
func GSI1Files(orgID, stage string) string {
	return OrgPK(orgID) + "#STAGE#" + stage
}
func GSI1Sessions(orgID string) string {
	return OrgPK(orgID) + "#DATE"
}
func GSI1Enrollments(orgID, contactID string) string {
	return OrgPK(orgID) + "#LEARNER#" + contactID
}

// GSI2 sert l'autocomplétion : préfixe de nom ou d'e-mail normalisé.
func GSI2Search(orgID string) string { return OrgPK(orgID) + "#SEARCH" }

// SearchKey normalise une valeur pour la recherche par préfixe : minuscules,
// espaces réduits, accents conservés (un utilisateur qui tape « Bertrand »
// doit trouver « Bertrand », et « bérard » doit trouver « Bérard » — la
// translittération viendra avec l'index de recherche, pas ici).
func SearchKey(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return strings.Join(cleaned, " ")
}

// NormalizeEmail met l'adresse en minuscules et supprime les espaces.
//
// Aucune autre normalisation : retirer les points d'une adresse Gmail, par
// exemple, refuserait à tort l'inscription de deux personnes distinctes chez
// un fournisseur qui les distingue.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// AuditPK est la clé de partition du journal : un sujet, une chaîne.
func AuditPK(subject string) string { return "SUBJECT#" + subject }

// AuditSK ordonne les événements d'un sujet par rang, sur une largeur fixe
// pour que le tri lexicographique de DynamoDB coïncide avec le tri numérique.
func AuditSK(seq int64) string { return fmt.Sprintf("%012d", seq) }
