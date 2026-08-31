package mail

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/lemlearn/api/internal/platform/ddb"
)

// JournalPK range les envois par mois.
//
// Par mois plutôt qu'en vrac : la question qu'on pose à ce journal est
// toujours « qu'est-ce qui est parti ces jours-ci », et une partition unique
// deviendrait le point chaud de la table le jour où le produit envoie
// sérieusement.
func JournalPK(at time.Time) string { return "MAIL#" + at.UTC().Format("2006-01") }

// Entry est la trace d'un envoi.
//
// Le corps du message n'y figure pas : il contient des liens de signature, des
// codes à usage unique, parfois un nom d'apprenant. Un journal consultable par
// l'équipe lemlearn n'a pas à porter cela — on garde de quoi répondre à « ce
// message est-il parti, quand, à qui », et rien de plus.
type Entry struct {
	ddb.Record

	SentAt   time.Time `dynamodbav:"sentAt" json:"sentAt"`
	To       string    `dynamodbav:"to" json:"to"`
	Subject  string    `dynamodbav:"subject" json:"subject"`
	Template string    `dynamodbav:"template,omitempty" json:"template,omitempty"`
	OrgID    string    `dynamodbav:"orgId,omitempty" json:"orgId,omitempty"`
	// Accepted dit que le fournisseur a pris le message en charge. Ce n'est
	// pas la même chose que reçu : sans notification de sa part, nous ne
	// savons pas si le message a été remis, rejeté ou classé indésirable, et
	// écrire « remis » revenait à l'affirmer sans le savoir.
	Delivered bool `dynamodbav:"delivered" json:"accepted"`
	// ProviderID nomme le message chez le fournisseur. C'est la seule prise
	// pour répondre à « il dit n'avoir rien reçu ».
	ProviderID string `dynamodbav:"providerId,omitempty" json:"providerId,omitempty"`
	Error      string `dynamodbav:"error,omitempty" json:"error,omitempty"`
	// Provider distingue un envoi réel d'un envoi journalisé en recette : lire
	// « livré » sur un environnement qui n'envoie rien serait un mensonge.
	Provider string `dynamodbav:"provider" json:"provider"`
}

// context key pour rattacher un envoi à son organisation et à son gabarit.
type journalKey struct{}

type journalContext struct {
	orgID    string
	template string
}

// WithContext rattache l'envoi à venir à une organisation et à un gabarit.
//
// Passer ces valeurs par le contexte évite d'ajouter deux paramètres à
// l'interface d'envoi, que quatre domaines implémentent ou consomment.
func WithContext(ctx context.Context, orgID, template string) context.Context {
	return context.WithValue(ctx, journalKey{}, journalContext{orgID: orgID, template: template})
}

// senderKey porte le nom d'expéditeur à afficher.
type senderKey struct{}

// WithSender fixe le nom sous lequel le message apparaît dans la boîte de
// réception.
//
// L'adresse technique, elle, ne change pas : elle dépend d'un domaine vérifié
// chez le fournisseur d'envoi, et un organisme ne peut pas en emprunter un
// autre sans le prouver. Mais c'est le nom qui s'affiche dans la liste des
// messages, et c'est lui que lit un stagiaire pour décider s'il ouvre.
func WithSender(ctx context.Context, name string) context.Context {
	if strings.TrimSpace(name) == "" {
		return ctx
	}
	return context.WithValue(ctx, senderKey{}, name)
}

// SenderFrom compose l'expéditeur à employer, à partir de celui configuré.
//
// Sans nom dans le contexte, l'adresse configurée part telle quelle.
func SenderFrom(ctx context.Context, configured string) string {
	name, _ := ctx.Value(senderKey{}).(string)
	if name == "" {
		return configured
	}
	// L'adresse est ce qui est entre chevrons, ou la chaîne entière quand
	// aucun nom n'a été configuré.
	adresse := configured
	if ouvrant := strings.LastIndex(configured, "<"); ouvrant >= 0 {
		adresse = strings.TrimSuffix(configured[ouvrant+1:], ">")
	}
	// Les guillemets et les chevrons casseraient l'en-tête : un nom
	// d'organisme est une donnée saisie, pas une constante du code.
	propre := strings.NewReplacer(`"`, "", "<", "", ">", "", "\r", "", "\n", "").Replace(name)
	return fmt.Sprintf("%s <%s>", strings.TrimSpace(propre), strings.TrimSpace(adresse))
}

func fromContext(ctx context.Context) journalContext {
	value, _ := ctx.Value(journalKey{}).(journalContext)
	return value
}

// Journaled enveloppe un expéditeur et consigne chaque envoi.
type Journaled struct {
	sender   Sender
	db       *ddb.Client
	provider string
	now      func() time.Time
}

// NewJournaled construit l'expéditeur journalisé.
func NewJournaled(sender Sender, db *ddb.Client, provider string, now func() time.Time) *Journaled {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Journaled{sender: sender, db: db, provider: provider, now: now}
}

// Send transmet le message puis le consigne, qu'il soit parti ou non.
//
// L'échec du journal ne fait pas échouer l'envoi : le message est déjà parti,
// et rendre une erreur ferait recommencer l'appelant — donc envoyer deux fois.
func (j *Journaled) Send(ctx context.Context, to, subject, html string) error {
	// L'identifiant du fournisseur n'est pas dans l'interface d'envoi : quatre
	// domaines l'implémentent ou la consomment, et l'élargir pour un besoin de
	// journal les toucherait tous. Ceux qui savent le rendre le déclarent.
	var providerID string
	var sendErr error
	if nommant, ok := j.sender.(interface {
		SendWithID(context.Context, string, string, string) (string, error)
	}); ok {
		providerID, sendErr = nommant.SendWithID(ctx, to, subject, html)
	} else {
		sendErr = j.sender.Send(ctx, to, subject, html)
	}

	if j.db != nil {
		now := j.now()
		meta := fromContext(ctx)
		entry := Entry{
			Record: ddb.Record{
				PK: JournalPK(now), SK: journalSK(now, to, subject),
				Type: "mail", CreatedAt: now, UpdatedAt: now,
				// Treize mois : la durée pendant laquelle un financeur peut
				// contester un dossier. Au-delà, la trace n'éclaire plus rien
				// et ne fait que porter des adresses.
				ExpiresAt: ddb.TTL(now.AddDate(1, 1, 0)),
			},
			SentAt: now, To: to, Subject: subject,
			Template: meta.template, OrgID: meta.orgID,
			Delivered: sendErr == nil, Provider: j.provider, ProviderID: providerID,
		}
		if sendErr != nil {
			entry.Error = sendErr.Error()
		}
		_ = ddb.Put(ctx, j.db, entry)
	}

	return sendErr
}

// journalSK ordonne les envois par instant, et reste unique.
func journalSK(at time.Time, to, subject string) string {
	sum := sha256.Sum256([]byte(to + "|" + subject))
	return fmt.Sprintf("%s#%s", at.UTC().Format("2006-01-02T15:04:05.000Z"), hex.EncodeToString(sum[:])[:8])
}

// Journal relit les envois.
type Journal struct{ db *ddb.Client }

// NewJournal construit le lecteur.
func NewJournal(db *ddb.Client) *Journal { return &Journal{db: db} }

// Recent renvoie les envois des deux derniers mois, du plus récent au plus
// ancien, filtrés par organisation ou par destinataire si demandé.
func (j *Journal) Recent(ctx context.Context, at time.Time, orgID, search string, limit int) ([]Entry, error) {
	if j.db == nil {
		return nil, nil
	}

	search = strings.ToLower(strings.TrimSpace(search))
	var entries []Entry

	// Le mois courant et le précédent : un envoi du 31 ne doit pas disparaître
	// de l'écran le 1er.
	for _, month := range []time.Time{at, at.AddDate(0, -1, 0)} {
		page, err := ddb.Query[Entry](ctx, j.db, ddb.QuerySpec{
			PK: JournalPK(month), Descending: true, Limit: int32(limit),
		})
		if err != nil {
			return nil, err
		}
		for _, entry := range page {
			if orgID != "" && entry.OrgID != orgID {
				continue
			}
			if search != "" &&
				!strings.Contains(strings.ToLower(entry.To), search) &&
				!strings.Contains(strings.ToLower(entry.Subject), search) {
				continue
			}
			entries = append(entries, entry)
			if len(entries) >= limit {
				return entries, nil
			}
		}
	}
	return entries, nil
}
