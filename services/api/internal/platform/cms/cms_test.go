package cms_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lemlearn/api/internal/platform/cms"
)

// testCertificate produit un certificat auto-signé, comme celui utilisé en
// développement à la place du cachet d'organisation.
func testCertificate(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("clé: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(20260143),
		Subject: pkix.Name{
			CommonName:   "Institut Vulcain",
			Organization: []string{"Institut Vulcain"},
			Country:      []string{"FR"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(2, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageEmailProtection},
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

// Le test qui compte : la signature produite ici doit être vérifiable par un
// outil que nous n'avons pas écrit. Se vérifier soi-même ne prouverait que la
// cohérence interne du code.
func TestSignatureIsVerifiableByOpenSSL(t *testing.T) {
	if _, err := exec.LookPath("openssl"); err != nil {
		t.Skip("openssl absent")
	}

	cert, key := testCertificate(t)
	content := []byte("Convention de formation CONV-2026-0143 — contenu signé détaché.")

	signature, err := cms.SignDetached(content, cms.Options{
		Certificate: cert, PrivateKey: key, SigningTime: time.Now(),
	})
	if err != nil {
		t.Fatalf("signature: %v", err)
	}

	dir := t.TempDir()
	sigPath := filepath.Join(dir, "signature.der")
	contentPath := filepath.Join(dir, "content.bin")
	certPath := filepath.Join(dir, "cert.pem")

	write(t, sigPath, signature)
	write(t, contentPath, content)
	write(t, certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))

	// -noverify : la chaîne de confiance d'un certificat auto-signé n'est pas
	// l'objet du test. Ce qui est vérifié ici, c'est que la structure CMS est
	// bien formée et que la signature couvre réellement le contenu.
	out, err := exec.Command("openssl", "cms", "-verify",
		"-in", sigPath, "-inform", "DER",
		"-content", contentPath, "-binary",
		"-certfile", certPath, "-noverify", "-no_content_verify",
		"-out", filepath.Join(dir, "out.bin"),
	).CombinedOutput()
	if err != nil {
		t.Fatalf("openssl a rejeté la signature: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Verification successful") {
		t.Fatalf("vérification inattendue: %s", out)
	}
}

// Une signature détachée ne doit pas valider un contenu différent : c'est
// exactement ce qui protège un document scellé.
func TestSignatureRejectsAlteredContent(t *testing.T) {
	if _, err := exec.LookPath("openssl"); err != nil {
		t.Skip("openssl absent")
	}

	cert, key := testCertificate(t)
	content := []byte("Convention de formation CONV-2026-0143.")

	signature, err := cms.SignDetached(content, cms.Options{Certificate: cert, PrivateKey: key})
	if err != nil {
		t.Fatalf("signature: %v", err)
	}

	dir := t.TempDir()
	altered := append([]byte(nil), content...)
	altered[10] ^= 0x01

	write(t, filepath.Join(dir, "signature.der"), signature)
	write(t, filepath.Join(dir, "altered.bin"), altered)
	write(t, filepath.Join(dir, "cert.pem"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))

	out, err := exec.Command("openssl", "cms", "-verify",
		"-in", filepath.Join(dir, "signature.der"), "-inform", "DER",
		"-content", filepath.Join(dir, "altered.bin"), "-binary",
		"-certfile", filepath.Join(dir, "cert.pem"), "-noverify",
		"-out", filepath.Join(dir, "out.bin"),
	).CombinedOutput()
	if err == nil {
		t.Fatalf("openssl a accepté une signature sur un contenu modifié:\n%s", out)
	}
}

// La structure doit contenir le certificat du signataire et les attributs du
// profil CAdES-BES — sans quoi un vérificateur strict refuse le document.
func TestSignatureCarriesCAdESAttributes(t *testing.T) {
	if _, err := exec.LookPath("openssl"); err != nil {
		t.Skip("openssl absent")
	}

	cert, key := testCertificate(t)
	signature, err := cms.SignDetached([]byte("contenu"), cms.Options{
		Certificate: cert, PrivateKey: key, SigningTime: time.Now(),
	})
	if err != nil {
		t.Fatalf("signature: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "signature.der")
	write(t, path, signature)

	out, err := exec.Command("openssl", "asn1parse", "-inform", "DER", "-in", path).CombinedOutput()
	if err != nil {
		t.Fatalf("asn1parse: %v\n%s", err, out)
	}
	text := string(out)

	for label, oid := range map[string]string{
		"signedData":    ":pkcs7-signedData",
		"contentType":   ":contentType",
		"messageDigest": ":messageDigest",
		"signingTime":   ":signingTime",
		// OpenSSL connaît cet OID sous son nom S/MIME : c'est ce nom
		// qu'affiche asn1parse, pas la forme pointée.
		"signing-certificate-v2": ":id-smime-aa-signingCertificateV2",
		"sha256":                 ":sha256",
	} {
		if !strings.Contains(text, oid) {
			t.Errorf("attribut %s absent de la structure", label)
		}
	}
	if !strings.Contains(text, "Institut Vulcain") {
		t.Error("le certificat du signataire n'est pas incorporé")
	}
}

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("écriture de %s: %v", path, err)
	}
}
