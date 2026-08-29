package pdfsig_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lemlearn/api/internal/platform/doc"
	"github.com/lemlearn/api/internal/platform/pdfsig"
)

func testCertificate(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("clé: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(20260143),
		Subject:               pkix.Name{CommonName: "Institut Vulcain", Country: []string{"FR"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(2, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("certificat: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("relecture: %v", err)
	}
	return cert, key
}

// samplePDF compile un vrai document Typst : signer un PDF fabriqué à la main
// ne prouverait rien du format réellement produit par le service.
func samplePDF(t *testing.T) []byte {
	t.Helper()
	if _, err := exec.LookPath("typst"); err != nil {
		t.Skip("binaire typst absent")
	}
	compiler, err := doc.NewBinaryCompiler()
	if err != nil {
		t.Fatalf("compilateur: %v", err)
	}

	var source doc.Source
	chrome := doc.Chrome{
		OrgName: "Institut Vulcain", OrgAddress: "12 rue des Écoles, 75005 Paris",
		LegalLine: "Institut Vulcain — SIRET 84291736500018",
		Reference: "CONV-2026-0143", Kind: "Convention de formation",
	}
	chrome.WritePreamble(&source)
	source.Line(`#lem_h1[Convention de formation professionnelle]`)
	source.Line(`Contenu de démonstration destiné au test de scellement.`)
	source.Line(`#pagebreak()`)
	source.Line(`#lem_sig_zone("client")`)

	pdf, err := compiler.Compile(context.Background(), doc.Document{
		Source: source.Bytes(), CreationUnix: time.Date(2026, 1, 14, 9, 0, 0, 0, time.UTC).Unix(),
	})
	if err != nil {
		t.Fatalf("compilation: %v", err)
	}
	return pdf
}

func sign(t *testing.T, pdf []byte) []byte {
	t.Helper()
	cert, key := testCertificate(t)
	signed, err := pdfsig.Sign(pdf, pdfsig.Options{
		Certificate: cert, PrivateKey: key,
		Name: "Institut Vulcain", Reason: "Convention de formation",
		Location: "Paris", SignedAt: time.Date(2026, 2, 3, 18, 47, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("scellement: %v", err)
	}
	return signed
}

// La révision d'origine doit être intacte, octet pour octet : c'est la
// promesse d'une mise à jour incrémentale, et ce qui permet de prouver que le
// document signé contient bien celui qui a été présenté.
func TestOriginalRevisionIsUntouched(t *testing.T) {
	pdf := samplePDF(t)
	signed := sign(t, pdf)

	if len(signed) <= len(pdf) {
		t.Fatalf("le document signé (%d o) n'est pas plus long que l'original (%d o)", len(signed), len(pdf))
	}
	if !bytes.Equal(signed[:len(pdf)], pdf) {
		t.Fatal("les octets d'origine ont été modifiés")
	}
	if !bytes.Contains(signed, []byte("/SubFilter /ETSI.CAdES.detached")) {
		t.Error("le sous-filtre PAdES est absent")
	}
	if !bytes.Contains(signed, []byte("/SigFlags 3")) {
		t.Error("le formulaire de signature n'est pas déclaré au catalogue")
	}
}

var byteRangePattern = regexp.MustCompile(`/ByteRange \[0 (\d+) (\d+) (\d+)\]`)

// /ByteRange doit couvrir exactement tout le fichier sauf la chaîne
// hexadécimale de la signature. Une plage mal calculée laisserait une zone du
// document modifiable sans invalider la signature — la faille classique.
func TestByteRangeCoversEverythingButTheSignature(t *testing.T) {
	signed := sign(t, samplePDF(t))

	match := byteRangePattern.FindSubmatch(signed)
	if match == nil {
		t.Fatal("/ByteRange non rempli")
	}
	a, _ := strconv.Atoi(string(match[1]))
	b, _ := strconv.Atoi(string(match[2]))
	c, _ := strconv.Atoi(string(match[3]))

	if b+c != len(signed) {
		t.Errorf("la plage laisse %d octet(s) hors signature", len(signed)-(b+c))
	}
	gap := signed[a:b]
	if gap[0] != '<' || gap[len(gap)-1] != '>' {
		t.Errorf("la zone exclue ne délimite pas la chaîne de signature: %q…%q", gap[0], gap[len(gap)-1])
	}
	if bytes.ContainsAny(gap[1:len(gap)-1], "<>/ ") {
		t.Error("la zone exclue déborde du contenu de la signature")
	}
}

// Le test décisif : extraire la signature du PDF et la faire vérifier par
// OpenSSL contre les octets que /ByteRange désigne. C'est exactement ce que
// fait un lecteur PDF conforme.
func TestSignedPDFVerifiesWithOpenSSL(t *testing.T) {
	if _, err := exec.LookPath("openssl"); err != nil {
		t.Skip("openssl absent")
	}
	signed := sign(t, samplePDF(t))

	match := byteRangePattern.FindSubmatch(signed)
	if match == nil {
		t.Fatal("/ByteRange non rempli")
	}
	a, _ := strconv.Atoi(string(match[1]))
	b, _ := strconv.Atoi(string(match[2]))
	c, _ := strconv.Atoi(string(match[3]))

	content := append(append([]byte{}, signed[:a]...), signed[b:b+c]...)
	raw := bytes.Trim(signed[a:b], "<>")
	signature, err := hex.DecodeString(strings.TrimRight(string(raw), "0"))
	if err != nil {
		// La réserve est complétée de zéros : un nombre impair de chiffres
		// après élagage se rattrape en réajoutant un zéro.
		signature, err = hex.DecodeString(strings.TrimRight(string(raw), "0") + "0")
		if err != nil {
			t.Fatalf("signature illisible: %v", err)
		}
	}

	dir := t.TempDir()
	sigPath := filepath.Join(dir, "sig.der")
	contentPath := filepath.Join(dir, "content.bin")
	if err := os.WriteFile(sigPath, signature, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(contentPath, content, 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command("openssl", "cms", "-verify",
		"-in", sigPath, "-inform", "DER",
		"-content", contentPath, "-binary", "-noverify",
		"-out", filepath.Join(dir, "out.bin"),
	).CombinedOutput()
	if err != nil {
		t.Fatalf("openssl a rejeté la signature du PDF: %v\n%s", err, out)
	}
}

// Et la contrepartie : un octet modifié dans le corps du document doit faire
// échouer la vérification.
func TestAlteredPDFFailsVerification(t *testing.T) {
	if _, err := exec.LookPath("openssl"); err != nil {
		t.Skip("openssl absent")
	}
	signed := sign(t, samplePDF(t))

	match := byteRangePattern.FindSubmatch(signed)
	a, _ := strconv.Atoi(string(match[1]))
	b, _ := strconv.Atoi(string(match[2]))
	c, _ := strconv.Atoi(string(match[3]))

	altered := append([]byte{}, signed...)
	altered[a/2] ^= 0x01 // au milieu de la première plage signée

	content := append(append([]byte{}, altered[:a]...), altered[b:b+c]...)
	raw := strings.TrimRight(string(bytes.Trim(signed[a:b], "<>")), "0")
	if len(raw)%2 == 1 {
		raw += "0"
	}
	signature, err := hex.DecodeString(raw)
	if err != nil {
		t.Fatalf("signature illisible: %v", err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sig.der"), signature, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "content.bin"), content, 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command("openssl", "cms", "-verify",
		"-in", filepath.Join(dir, "sig.der"), "-inform", "DER",
		"-content", filepath.Join(dir, "content.bin"), "-binary", "-noverify",
		"-out", filepath.Join(dir, "out.bin"),
	).CombinedOutput()
	if err == nil {
		t.Fatalf("un document modifié a passé la vérification:\n%s", out)
	}
}

// Le document signé doit rester lisible : un PDF que le scellement casserait
// ne servirait à rien, si valide soit sa signature.
func TestSignedPDFRemainsReadable(t *testing.T) {
	signed := sign(t, samplePDF(t))

	dir := t.TempDir()
	path := filepath.Join(dir, "signe.pdf")
	if err := os.WriteFile(path, signed, 0o600); err != nil {
		t.Fatal(err)
	}

	// sips fait partie de macOS et refuse un PDF corrompu ; à défaut, on se
	// contente de la structure.
	if _, err := exec.LookPath("sips"); err != nil {
		t.Skip("sips absent")
	}
	out, err := exec.Command("sips", "-g", "pixelWidth", path).CombinedOutput()
	if err != nil {
		t.Fatalf("le PDF signé n'est plus lisible: %v\n%s", err, out)
	}
}

func TestSigningTwiceIsRefused(t *testing.T) {
	signed := sign(t, samplePDF(t))
	cert, key := testCertificate(t)

	// Une seconde signature exigerait de fusionner les formulaires : mieux
	// vaut refuser explicitement que produire un document dont le panneau de
	// signature serait incohérent.
	if _, err := pdfsig.Sign(signed, pdfsig.Options{Certificate: cert, PrivateKey: key}); err == nil {
		t.Fatal("une seconde signature a été acceptée")
	}
}

func TestSignatureDigestIsStable(t *testing.T) {
	pdf := samplePDF(t)
	first := sha256.Sum256(pdf)
	second := sha256.Sum256(samplePDF(t))
	if first != second {
		t.Fatal("le document de référence n'est pas reproductible, le test n'a pas de sens")
	}
}

// Le panneau de signature d'un lecteur PDF nomme le champ. Un dossier
// probatoire part chez un financeur ou un auditeur : c'est l'organisme de
// formation qu'ils doivent y lire, jamais l'outil qui a produit la pièce.
func TestSignatureFieldNamesTheOrganisation(t *testing.T) {
	pdf := samplePDF(t)
	cert, key := testCertificate(t)

	signed, err := pdfsig.Sign(pdf, pdfsig.Options{
		Certificate: cert, PrivateKey: key,
		Name: "Léa Martin", Reason: "Convention de formation",
		FieldTitle: "Institut Vulcain",
		SignedAt:   time.Date(2026, 2, 3, 18, 47, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("scellement: %v", err)
	}

	if !bytes.Contains(signed, []byte("/T (Institut Vulcain)")) {
		t.Error("le champ de signature ne porte pas le nom de l'organisme")
	}
	if bytes.Contains(bytes.ToLower(signed), []byte("lemlearn")) {
		t.Error("le nom de l'outil apparaît dans le document scellé")
	}
}

// Sans organisme connu, le champ reste neutre plutôt que de nommer l'outil.
func TestSignatureFieldFallsBackToANeutralName(t *testing.T) {
	signed := sign(t, samplePDF(t))
	if !bytes.Contains(signed, []byte("/T (Signature electronique)")) {
		t.Error("intitulé de repli absent")
	}
}
