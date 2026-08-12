package docflow_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/lemlearn/api/internal/crm"
	"github.com/lemlearn/api/internal/docflow"
	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/blob"
	"github.com/lemlearn/api/internal/platform/ddb"
	"github.com/lemlearn/api/internal/platform/doc"
	"github.com/lemlearn/api/internal/platform/mail"
	"github.com/lemlearn/api/internal/signature"
)

type stack struct {
	db      *ddb.Client
	crm     *crm.Service
	sign    *signature.Service
	blobs   *blob.Memory
	mailer  *mail.Log
	org     identity.Org
	file    crm.File
	actor   audit.Actor
	learner crm.Contact
}

func newStack(t *testing.T) stack {
	t.Helper()
	if _, err := exec.LookPath("typst"); err != nil {
		t.Skip("binaire typst absent : `brew install typst`")
	}

	ctx := context.Background()
	db := ddb.NewTestClient(t)
	ident := identity.NewService(db, nil)
	crmService := crm.NewService(db, nil)

	compiler, err := doc.NewBinaryCompiler()
	if err != nil {
		t.Fatalf("compilateur: %v", err)
	}

	org, owner, err := ident.Register(ctx, identity.RegisterInput{
		OrgName: "Institut Vulcain", Email: "marie@vulcain.fr",
		Password: "correcte-agrafe-cheval-pile", FirstName: "Marie", LastName: "Dubreuil",
	})
	if err != nil {
		t.Fatalf("inscription: %v", err)
	}

	actor := audit.Actor{Type: audit.ActorUser, ID: owner.ID, Label: owner.FullName(), IP: "82.65.14.3"}

	learner := crm.NewContact(org.ID, crm.KindLearner, time.Now().UTC())
	learner.FirstName = "Léa"
	learner.LastName = "Bertrand"
	learner.Email = "lea.bertrand@example.fr"
	learner.Address = crm.Address{Line1: "8 avenue Foch", PostalCode: "69006", City: "Lyon"}
	learner, err = crmService.CreateContact(ctx, learner)
	if err != nil {
		t.Fatalf("apprenant: %v", err)
	}

	file, err := crmService.CreateFile(ctx, crm.CreateFileInput{
		OrgID: org.ID, Title: "Sécurité incendie — SSIAP 1",
		LearnerID: learner.ID, PriceHT: 1250, Actor: actor,
	})
	if err != nil {
		t.Fatalf("dossier: %v", err)
	}

	blobs := blob.NewMemory()
	mailer := mail.NewLog(nil)

	return stack{
		db: db, crm: crmService, blobs: blobs, mailer: mailer,
		org: org, file: file, actor: actor, learner: learner,
		sign: signature.NewService(signature.Deps{
			DB:       db,
			Renderer: docflow.NewRenderer(ident, crmService, compiler),
			Blobs:    blobs,
			Mailer:   mailer,
			AppURL:   "https://app.lemlearn.fr",
		}),
	}
}

func (s stack) issue(t *testing.T) (signature.Request, string) {
	t.Helper()
	req, token, err := s.sign.Issue(context.Background(), signature.IssueInput{
		OrgID: s.org.ID, FileID: s.file.ID,
		Kind: "convention", Reference: "CONV-2026-0143",
		Role:       doc.RoleClient,
		SignerName: "Léa Bertrand", SignerEmail: "lea.bertrand@example.fr",
		Actor: s.actor,
	})
	if err != nil {
		t.Fatalf("émission: %v", err)
	}
	return req, token
}

// otpFromMail relit le code dans le courriel capturé — c'est exactement ce que
// fera le signataire.
func (s stack) otpFromMail(t *testing.T) string {
	t.Helper()
	sent := s.mailer.Sent()
	if len(sent) == 0 {
		t.Fatal("aucun courriel envoyé")
	}
	last := sent[len(sent)-1]
	code := regexp.MustCompile(`\b\d{6}\b`).FindString(last.Subject)
	if code == "" {
		t.Fatalf("aucun code dans le sujet %q", last.Subject)
	}
	return code
}

// drawing produit un PNG de tracé plausible.
func drawing(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 320, 120))
	for x := 10; x < 310; x++ {
		y := 60 + int(20*float64((x%40))/40)
		for thickness := range 3 {
			img.Set(x, y+thickness, color.RGBA{16, 19, 26, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png: %v", err)
	}
	return buf.Bytes()
}

func confirmInput(t *testing.T, otp string) signature.ConfirmInput {
	t.Helper()
	return signature.ConfirmInput{
		OTP:         otp,
		Mention:     "Lu et approuvé, bon pour accord",
		DrawingPNG:  drawing(t),
		StrokeCount: 3,
		DurationMs:  2400,
		IP:          "78.192.44.10",
		UserAgent:   "Mozilla/5.0 (iPhone)",
	}
}

// Le parcours complet : émission, ouverture, code, signature, scellement.
func TestSignatureHappyPath(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()

	req, token := s.issue(t)
	if req.UnsignedSHA256 == "" {
		t.Fatal("l'empreinte du document vierge n'a pas été figée")
	}
	if len(s.mailer.Sent()) != 1 {
		t.Fatalf("%d courriel(s) d'invitation", len(s.mailer.Sent()))
	}

	opened, unsigned, err := s.sign.Open(ctx, token, "78.192.44.10", "Mozilla/5.0 (iPhone)")
	if err != nil {
		t.Fatalf("ouverture: %v", err)
	}
	if opened.Status != signature.StatusOpened {
		t.Errorf("statut après ouverture = %q", opened.Status)
	}
	if !bytes.HasPrefix(unsigned, []byte("%PDF")) {
		t.Fatal("le document remis n'est pas un PDF")
	}

	if _, err := s.sign.SendOTP(ctx, token, "78.192.44.10", "Mozilla/5.0 (iPhone)"); err != nil {
		t.Fatalf("envoi du code: %v", err)
	}

	signed, err := s.sign.Confirm(ctx, token, confirmInput(t, s.otpFromMail(t)))
	if err != nil {
		t.Fatalf("signature: %v", err)
	}
	if signed.Status != signature.StatusSigned || signed.Proof == nil {
		t.Fatalf("statut %q, preuve %v", signed.Status, signed.Proof)
	}

	// Le document signé doit différer du vierge — la signature y est rendue —
	// tout en restant vérifiable par son empreinte.
	sealed, err := s.sign.Sealed(ctx, signed)
	if err != nil {
		t.Fatalf("relecture du document scellé: %v", err)
	}
	if bytes.Equal(sealed, unsigned) {
		t.Fatal("le document signé est identique au document vierge")
	}
	sum := sha256.Sum256(sealed)
	if hex.EncodeToString(sum[:]) != signed.Proof.SealedSHA256 {
		t.Fatal("l'empreinte du document scellé ne correspond pas à celle journalisée")
	}

	// Le dossier de preuve doit être complet : c'est ce qui est opposable.
	proof := signed.Proof
	if proof.IP == "" || proof.UserAgent == "" || proof.Mention == "" ||
		proof.DrawingSHA256 == "" || proof.StrokeCount == 0 {
		t.Errorf("dossier de preuve incomplet: %+v", proof)
	}
}

// Le journal du dossier doit raconter toute l'histoire, dans l'ordre, et
// rester vérifiable.
func TestSignatureAuditTrail(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()

	_, token := s.issue(t)
	if _, _, err := s.sign.Open(ctx, token, "78.192.44.10", "iPhone"); err != nil {
		t.Fatalf("ouverture: %v", err)
	}
	if _, err := s.sign.SendOTP(ctx, token, "78.192.44.10", "iPhone"); err != nil {
		t.Fatalf("code: %v", err)
	}
	// Un essai raté avant le bon : il doit figurer au journal.
	if _, err := s.sign.Confirm(ctx, token, confirmInput(t, "000000")); !errors.Is(err, signature.ErrOTPInvalid) {
		t.Fatalf("code faux accepté: %v", err)
	}
	if _, err := s.sign.Confirm(ctx, token, confirmInput(t, s.otpFromMail(t))); err != nil {
		t.Fatalf("signature: %v", err)
	}

	events, err := s.crm.Timeline(ctx, s.file.ID)
	if err != nil {
		t.Fatalf("journal: %v", err)
	}

	want := []audit.Action{
		audit.ActionFileCreated,
		audit.ActionDocumentSent,
		audit.ActionSignatureOpened,
		audit.ActionSignatureOTPSent,
		audit.ActionSignatureOTPFail,
		audit.ActionSignatureOTPOK,
		audit.ActionDocumentSigned,
	}
	if len(events) != len(want) {
		var got []string
		for _, e := range events {
			got = append(got, string(e.Action))
		}
		t.Fatalf("journal inattendu:\n  obtenu %v\n  attendu %v", got, want)
	}
	for i, action := range want {
		if events[i].Action != action {
			t.Errorf("rang %d : %q, attendu %q", i+1, events[i].Action, action)
		}
	}

	// Le code ne doit apparaître nulle part au journal — ni en clair, ni
	// dans une charge utile « pour le débogage ».
	code := s.otpFromMail(t)
	for _, event := range events {
		for key, value := range event.Payload {
			if text, ok := value.(string); ok && strings.Contains(text, code) {
				t.Errorf("le code apparaît au journal (%s.%s)", event.Action, key)
			}
		}
	}

	if err := audit.Verify(events); err != nil {
		t.Fatalf("chaîne invalide: %v", err)
	}
}

// Trois codes faux épuisent la tentative : au-delà, il faut redemander un code.
func TestOTPAttemptsAreExhausted(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()

	_, token := s.issue(t)
	if _, err := s.sign.SendOTP(ctx, token, "1.2.3.4", "test"); err != nil {
		t.Fatalf("code: %v", err)
	}

	for attempt := 1; attempt <= signature.MaxOTPAttempts; attempt++ {
		_, err := s.sign.Confirm(ctx, token, confirmInput(t, "000000"))
		if attempt < signature.MaxOTPAttempts && !errors.Is(err, signature.ErrOTPInvalid) {
			t.Fatalf("essai %d: %v", attempt, err)
		}
		if attempt == signature.MaxOTPAttempts && !errors.Is(err, signature.ErrOTPExhausted) {
			t.Fatalf("essai %d: %v, attendu épuisement", attempt, err)
		}
	}

	// Même le bon code ne passe plus : sans cela, trois essais deviendraient
	// une infinité d'essais.
	if _, err := s.sign.Confirm(ctx, token, confirmInput(t, s.otpFromMail(t))); !errors.Is(err, signature.ErrOTPExhausted) {
		t.Fatalf("le code reste utilisable après épuisement: %v", err)
	}
}

// Un tracé indigent — un clic — n'est pas une signature manuscrite.
func TestPoorDrawingIsRejected(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()

	_, token := s.issue(t)
	if _, err := s.sign.SendOTP(ctx, token, "1.2.3.4", "test"); err != nil {
		t.Fatalf("code: %v", err)
	}

	in := confirmInput(t, s.otpFromMail(t))
	in.DurationMs = 40
	in.StrokeCount = 1
	if _, err := s.sign.Confirm(ctx, token, in); !errors.Is(err, signature.ErrDrawingTooPoor) {
		t.Fatalf("tracé indigent accepté: %v", err)
	}
}

// Un document ne se signe qu'une fois.
func TestDoubleSignatureIsRefused(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()

	_, token := s.issue(t)
	if _, err := s.sign.SendOTP(ctx, token, "1.2.3.4", "test"); err != nil {
		t.Fatalf("code: %v", err)
	}
	if _, err := s.sign.Confirm(ctx, token, confirmInput(t, s.otpFromMail(t))); err != nil {
		t.Fatalf("première signature: %v", err)
	}

	if _, err := s.sign.SendOTP(ctx, token, "1.2.3.4", "test"); !errors.Is(err, signature.ErrAlreadySigned) {
		t.Fatalf("un second code a été émis: %v", err)
	}
}

// Un jeton inconnu ne révèle rien.
func TestUnknownTokenIsNotFound(t *testing.T) {
	s := newStack(t)

	if _, err := s.sign.Resolve(context.Background(), "jeton-inexistant"); !errors.Is(err, signature.ErrNotFound) {
		t.Fatalf("jeton inconnu: %v", err)
	}
}

// Le cas qui protège la valeur juridique : si le document archivé est modifié
// après scellement, la vérification doit échouer.
func TestTamperedSealedDocumentIsDetected(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()

	_, token := s.issue(t)
	if _, err := s.sign.SendOTP(ctx, token, "1.2.3.4", "test"); err != nil {
		t.Fatalf("code: %v", err)
	}
	signed, err := s.sign.Confirm(ctx, token, confirmInput(t, s.otpFromMail(t)))
	if err != nil {
		t.Fatalf("signature: %v", err)
	}

	sealed, err := s.blobs.Get(ctx, signed.Proof.SealedKey)
	if err != nil {
		t.Fatalf("relecture: %v", err)
	}
	// Un seul octet modifié, au milieu du fichier.
	sealed[len(sealed)/2] ^= 0x01
	if err := s.blobs.Put(ctx, signed.Proof.SealedKey, "application/pdf", sealed); err != nil {
		t.Fatalf("réécriture: %v", err)
	}

	if _, err := s.sign.Sealed(ctx, signed); err == nil {
		t.Fatal("un document altéré après scellement a passé la vérification")
	} else if !strings.Contains(err.Error(), "intégrité") {
		t.Errorf("message peu explicite: %v", err)
	}
}
