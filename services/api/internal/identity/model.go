// Package identity gère les organisations, leurs utilisateurs et leurs
// sessions.
//
// Un compte appartient à une et une seule organisation : un formateur qui
// intervient pour deux organismes a deux comptes. C'est délibéré — la
// frontière d'isolation du produit est l'organisation, et un compte à cheval
// sur deux partitions rendrait cette frontière négociable.
package identity

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/lemlearn/api/internal/platform/ddb"
)

// Role détermine ce qu'un utilisateur peut faire.
type Role string

const (
	// RoleOwner : propriétaire de l'organisation, seul à pouvoir gérer
	// l'abonnement et supprimer l'organisation.
	RoleOwner Role = "owner"
	// RoleAdmin : gestion complète du CRM, du catalogue et des preuves.
	RoleAdmin Role = "admin"
	// RoleTrainer : ses sessions uniquement, émargement et évaluations.
	RoleTrainer Role = "trainer"
	// RoleLearner : son espace apprenant.
	RoleLearner Role = "learner"
	// RoleSuperAdmin : équipe lemlearn. N'appartient à aucune organisation
	// cliente et n'accède aux données qu'en impersonation tracée.
	RoleSuperAdmin Role = "superadmin"
)

// Valid indique si le rôle fait partie de la liste fermée.
func (r Role) Valid() bool {
	switch r {
	case RoleOwner, RoleAdmin, RoleTrainer, RoleLearner, RoleSuperAdmin:
		return true
	}
	return false
}

// CanManageCRM autorise la gestion des contacts, dossiers et documents.
func (r Role) CanManageCRM() bool {
	return r == RoleOwner || r == RoleAdmin || r == RoleSuperAdmin
}

// CanTeach autorise l'émargement et la correction.
func (r Role) CanTeach() bool {
	return r.CanManageCRM() || r == RoleTrainer
}

// Org est un organisme de formation client.
type Org struct {
	ddb.Record

	ID   string `dynamodbav:"id"`
	Name string `dynamodbav:"name"`
	// SIRET et NDA (numéro de déclaration d'activité) figurent sur tous les
	// documents contractuels : ils sont portés par l'organisation, pas
	// ressaisis à chaque convention.
	SIRET      string `dynamodbav:"siret,omitempty"`
	NDA        string `dynamodbav:"nda,omitempty"`
	Address    string `dynamodbav:"address,omitempty"`
	PostalCode string `dynamodbav:"postalCode,omitempty"`
	City       string `dynamodbav:"city,omitempty"`
	// Plan de l'abonnement, piloté depuis la vue super-admin.
	Plan string `dynamodbav:"plan"`
	// QualiopiCertified conditionne l'affichage du tableau de bord de
	// conformité et les relances de satisfaction à froid.
	QualiopiCertified bool `dynamodbav:"qualiopiCertified"`
}

// NewOrg construit une organisation prête à écrire.
func NewOrg(name string, now time.Time) Org {
	id := NewID()
	return Org{
		Record: ddb.Record{
			PK:        ddb.OrgPK(id),
			SK:        ddb.OrgMetaSK,
			Type:      "org",
			CreatedAt: now,
			UpdatedAt: now,
		},
		ID:   id,
		Name: name,
		Plan: "trial",
	}
}

// User est un compte humain rattaché à une organisation.
type User struct {
	ddb.Record

	ID        string `dynamodbav:"id"`
	OrgID     string `dynamodbav:"orgId"`
	Email     string `dynamodbav:"email"`
	FirstName string `dynamodbav:"firstName"`
	LastName  string `dynamodbav:"lastName"`
	Role      Role   `dynamodbav:"role"`
	// ContactID relie un compte apprenant à sa fiche du CRM. Sans lui,
	// l'espace apprenant ne saurait pas de quelles inscriptions il parle :
	// l'apprenant est un contact côté gestion, un utilisateur côté connexion,
	// et les deux doivent se rejoindre quelque part.
	ContactID string `dynamodbav:"contactId,omitempty"`
	// PasswordHash est au format PHC ($argon2id$…). Il n'est jamais renvoyé
	// par l'API : voir Public().
	PasswordHash string     `dynamodbav:"passwordHash"`
	Disabled     bool       `dynamodbav:"disabled"`
	LastLoginAt  *time.Time `dynamodbav:"lastLoginAt,omitempty"`
}

// FullName est le nom affiché.
func (u User) FullName() string {
	if u.FirstName == "" {
		return u.LastName
	}
	return u.FirstName + " " + u.LastName
}

// PublicUser est la projection renvoyée par l'API. Le type est distinct pour
// que l'empreinte du mot de passe ne puisse pas fuir par ajout d'un champ à
// User : il faudrait l'ajouter ici aussi, ce qui se voit en revue.
type PublicUser struct {
	ID        string `json:"id"`
	OrgID     string `json:"orgId"`
	ContactID string `json:"contactId,omitempty"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Role      Role   `json:"role"`
}

// Public projette l'utilisateur pour l'API.
func (u User) Public() PublicUser {
	return PublicUser{
		ID: u.ID, OrgID: u.OrgID, ContactID: u.ContactID, Email: u.Email,
		FirstName: u.FirstName, LastName: u.LastName, Role: u.Role,
	}
}

// NewUser construit un utilisateur prêt à écrire.
func NewUser(orgID, email, firstName, lastName string, role Role, passwordHash string, now time.Time) User {
	id := NewID()
	email = ddb.NormalizeEmail(email)
	return User{
		Record: ddb.Record{
			PK:     ddb.OrgPK(orgID),
			SK:     ddb.UserSK(id),
			GSI1PK: ddb.OrgPK(orgID) + "#KIND#user",
			GSI1SK: ddb.SearchKey(lastName, firstName),
			Type:   "user",

			CreatedAt: now,
			UpdatedAt: now,
		},
		ID: id, OrgID: orgID, Email: email,
		FirstName: firstName, LastName: lastName,
		Role: role, PasswordHash: passwordHash,
	}
}

// EmailPointer réserve une adresse e-mail au niveau global.
//
// Écrit sous condition d'inexistence dans la même transaction que
// l'utilisateur : deux inscriptions simultanées avec la même adresse ne
// peuvent pas réussir toutes les deux.
type EmailPointer struct {
	ddb.Record

	Email  string `dynamodbav:"email"`
	OrgID  string `dynamodbav:"orgId"`
	UserID string `dynamodbav:"userId"`
}

// NewEmailPointer construit la réservation d'une adresse.
func NewEmailPointer(email, orgID, userID string, now time.Time) EmailPointer {
	email = ddb.NormalizeEmail(email)
	return EmailPointer{
		Record: ddb.Record{
			PK:        ddb.EmailPointerPK(email),
			SK:        ddb.EmailPointerSK,
			Type:      "email_pointer",
			CreatedAt: now,
			UpdatedAt: now,
		},
		Email: email, OrgID: orgID, UserID: userID,
	}
}

// Session est une session authentifiée.
//
// Le jeton est opaque et révocable, plutôt qu'un JWT : révoquer un JWT impose
// de toute façon une liste de révocation consultée à chaque requête, donc un
// aller-retour en base — autant stocker la session et pouvoir la couper
// réellement, ce qui est indispensable pour l'impersonation.
type Session struct {
	ddb.Record

	// TokenHash est le SHA-256 du jeton. Le jeton lui-même n'existe que dans
	// le cookie du navigateur : une copie de la base ne permet pas de rejouer
	// les sessions actives.
	TokenHash string    `dynamodbav:"tokenHash"`
	UserID    string    `dynamodbav:"userId"`
	OrgID     string    `dynamodbav:"orgId"`
	Role      Role      `dynamodbav:"role"`
	IssuedAt  time.Time `dynamodbav:"issuedAt"`
	ExpiresIn time.Time `dynamodbav:"expiresInAt"`
	IP        string    `dynamodbav:"ip,omitempty"`
	UserAgent string    `dynamodbav:"userAgent,omitempty"`
	// ImpersonatedBy porte l'identifiant du super-administrateur lorsqu'il
	// agit à la place d'un client. Toute action de la session est journalisée
	// avec ce champ : une impersonation ne peut pas être discrète.
	ImpersonatedBy string `dynamodbav:"impersonatedBy,omitempty"`
}

// SessionTTL est la durée de vie d'une session inactive.
const SessionTTL = 12 * time.Hour

// NewSession construit une session à partir de l'empreinte d'un jeton.
func NewSession(tokenHash string, user User, ip, userAgent, impersonatedBy string, now time.Time) Session {
	expires := now.Add(SessionTTL)
	return Session{
		Record: ddb.Record{
			PK:        ddb.AuthSessionPK(tokenHash),
			SK:        ddb.AuthSessionSK,
			Type:      "session",
			CreatedAt: now,
			UpdatedAt: now,
			ExpiresAt: ddb.TTL(expires),
		},
		TokenHash: tokenHash,
		UserID:    user.ID, OrgID: user.OrgID, Role: user.Role,
		IssuedAt: now, ExpiresIn: expires,
		IP: ip, UserAgent: userAgent, ImpersonatedBy: impersonatedBy,
	}
}

// Expired indique si la session ne doit plus être acceptée.
//
// Le contrôle est fait en Go et pas seulement par le TTL DynamoDB : le TTL
// supprime les éléments avec un délai pouvant atteindre plusieurs heures, ce
// qui est acceptable pour le ménage mais pas pour l'autorisation.
func (s Session) Expired(now time.Time) bool {
	return !now.Before(s.ExpiresIn)
}

// NewID produit un ULID : trié par date de création et sans collision, ce qui
// permet de l'utiliser directement comme clé de tri.
func NewID() string {
	return ulid.MustNew(ulid.Now(), rand.Reader).String()
}

// ErrInvalidCredentials couvre indifféremment « e-mail inconnu » et « mot de
// passe faux » : les distinguer permettrait d'énumérer les comptes existants.
var ErrInvalidCredentials = fmt.Errorf("identifiants invalides")

// ErrEmailTaken signale une adresse déjà utilisée.
var ErrEmailTaken = fmt.Errorf("adresse e-mail déjà utilisée")

// ErrDisabled signale un compte désactivé.
var ErrDisabled = fmt.Errorf("compte désactivé")
