package emailtpl

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lemlearn/api/internal/platform/ddb"
	"github.com/lemlearn/api/internal/platform/mail"
)

// PlatformPK range les gabarits hors de toute organisation.
//
// Ils appartiennent au produit, pas à un client : un organisme qui réécrirait
// le courriel de code de signature pourrait en retirer l'avertissement de
// sécurité, sur un message que nous envoyons en son nom.
const PlatformPK = "PLATFORM"

// Override est un gabarit réécrit par l'équipe.
type Override struct {
	ddb.Record

	Key     string `dynamodbav:"key" json:"key"`
	Subject string `dynamodbav:"subject" json:"subject"`
	Body    string `dynamodbav:"body" json:"body"`
	// By nomme l'auteur de la dernière modification : un message envoyé à des
	// milliers de personnes ne doit pas changer anonymement.
	By string `dynamodbav:"by,omitempty" json:"by,omitempty"`
}

// Service résout un gabarit : la version réécrite si elle existe, celle du
// code sinon.
type Service struct {
	db  *ddb.Client
	now func() time.Time
}

// NewService construit le service. Un db nil est accepté : le produit rend
// alors ses gabarits d'origine, ce qui est exactement ce qu'il faut en local.
func NewService(db *ddb.Client, now func() time.Time) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{db: db, now: now}
}

func overrideSK(key string) string { return "EMAIL#" + key }

// Compose rend un courriel prêt à partir.
//
// Un gabarit réécrit qui ne s'exécute pas ne fait pas échouer l'envoi : on
// retombe sur celui d'origine. Un lien de signature ne doit pas rester bloqué
// parce que quelqu'un a laissé une accolade ouverte trois semaines plus tôt.
func (s *Service) Compose(ctx context.Context, key string, data map[string]any) (mail.Composed, error) {
	definition, ok := DefaultFor(key)
	if !ok {
		return mail.Composed{}, fmt.Errorf("gabarit %q inconnu", key)
	}

	// L'identité est commune à tous les messages : la compléter ici évite
	// qu'un appelant l'oublie et qu'un courriel parte sans enseigne — ou pire,
	// avec un gabarit qui échoue au rendu parce qu'un champ manque.
	//
	// Les valeurs de repli ne sont pas celles de lemlearn : un message dont
	// l'organisme est inconnu vaut mieux neutre que faussement attribué.
	if data == nil {
		data = map[string]any{}
	}
	for champ, repli := range map[string]any{
		"LogoURL":     "",
		"BrandName":   "Votre organisme de formation",
		"BrandAccent": "#4B37B8",
		"BrandInk":    "#FFFFFF",
	} {
		if valeur, given := data[champ]; !given || valeur == nil || valeur == "" {
			data[champ] = repli
		}
	}

	subject, body := definition.Subject, definition.Body
	if s.db != nil {
		override, err := ddb.Get[Override](ctx, s.db, PlatformPK, overrideSK(key))
		if err == nil {
			if renderedSubject, renderedBody, err := Render(override.Subject, override.Body, data); err == nil {
				return mail.Composed{Subject: renderedSubject, HTML: renderedBody}, nil
			}
			// Le gabarit réécrit est cassé : on le signale par l'erreur de
			// retour, mais on envoie quand même celui d'origine.
			defer func() { _ = err }()
		} else if !errors.Is(err, ddb.ErrNotFound) {
			return mail.Composed{}, err
		}
	}

	renderedSubject, renderedBody, err := Render(subject, body, data)
	if err != nil {
		return mail.Composed{}, err
	}
	return mail.Composed{Subject: renderedSubject, HTML: renderedBody}, nil
}

// Current renvoie le gabarit en vigueur et celui d'origine.
type Current struct {
	Definition
	Overridden bool       `json:"overridden"`
	UpdatedAt  *time.Time `json:"updatedAt,omitempty"`
	UpdatedBy  string     `json:"updatedBy,omitempty"`
	// DefaultSubject et DefaultBody permettent de montrer l'écart et de
	// revenir en arrière sans consulter le dépôt.
	DefaultSubject string `json:"defaultSubject"`
	DefaultBody    string `json:"defaultBody"`
}

// List renvoie l'état de tous les gabarits.
func (s *Service) List(ctx context.Context) ([]Current, error) {
	overrides := map[string]Override{}
	if s.db != nil {
		stored, err := ddb.Query[Override](ctx, s.db, ddb.QuerySpec{PK: PlatformPK, SKPrefix: "EMAIL#"})
		if err != nil {
			return nil, err
		}
		for _, override := range stored {
			overrides[override.Key] = override
		}
	}

	definitions := Defaults()
	list := make([]Current, 0, len(definitions))
	for _, definition := range definitions {
		list = append(list, merge(definition, overrides[definition.Key]))
	}
	return list, nil
}

// Get renvoie l'état d'un gabarit.
func (s *Service) Get(ctx context.Context, key string) (Current, error) {
	definition, ok := DefaultFor(key)
	if !ok {
		return Current{}, fmt.Errorf("gabarit %q inconnu", key)
	}
	if s.db == nil {
		return merge(definition, Override{}), nil
	}

	override, err := ddb.Get[Override](ctx, s.db, PlatformPK, overrideSK(key))
	if err != nil && !errors.Is(err, ddb.ErrNotFound) {
		return Current{}, err
	}
	return merge(definition, override), nil
}

func merge(definition Definition, override Override) Current {
	current := Current{
		Definition:     definition,
		DefaultSubject: definition.Subject,
		DefaultBody:    definition.Body,
	}
	if override.Key == "" {
		return current
	}

	current.Subject = override.Subject
	current.Body = override.Body
	current.Overridden = true
	current.UpdatedBy = override.By
	updated := override.UpdatedAt
	current.UpdatedAt = &updated
	return current
}

// Save enregistre une réécriture.
//
// Le gabarit est rendu avec le jeu de valeurs de démonstration avant d'être
// accepté : une erreur de syntaxe se découvre ici, pas au moment où un
// signataire attend son code.
func (s *Service) Save(ctx context.Context, key, subject, body, by string) (Current, error) {
	definition, ok := DefaultFor(key)
	if !ok {
		return Current{}, fmt.Errorf("gabarit %q inconnu", key)
	}
	if s.db == nil {
		return Current{}, fmt.Errorf("les gabarits ne sont pas modifiables sans base de données")
	}
	if _, _, err := Render(subject, body, definition.Sample()); err != nil {
		return Current{}, err
	}

	now := s.now()
	override := Override{
		Record: ddb.Record{
			PK: PlatformPK, SK: overrideSK(key), Type: "email_template",
			CreatedAt: now, UpdatedAt: now,
		},
		Key: key, Subject: subject, Body: body, By: by,
	}
	if err := ddb.Put(ctx, s.db, override); err != nil {
		return Current{}, err
	}
	return merge(definition, override), nil
}

// Reset revient au gabarit d'origine.
func (s *Service) Reset(ctx context.Context, key string) (Current, error) {
	definition, ok := DefaultFor(key)
	if !ok {
		return Current{}, fmt.Errorf("gabarit %q inconnu", key)
	}
	if s.db != nil {
		if err := ddb.Delete(ctx, s.db, PlatformPK, overrideSK(key)); err != nil &&
			!errors.Is(err, ddb.ErrNotFound) {
			return Current{}, err
		}
	}
	return merge(definition, Override{}), nil
}

// Preview rend un gabarit sans l'enregistrer.
func Preview(key, subject, body string) (mail.Composed, error) {
	definition, ok := DefaultFor(key)
	if !ok {
		return mail.Composed{}, fmt.Errorf("gabarit %q inconnu", key)
	}
	renderedSubject, renderedBody, err := Render(subject, body, definition.Sample())
	if err != nil {
		return mail.Composed{}, err
	}
	return mail.Composed{Subject: renderedSubject, HTML: renderedBody}, nil
}
