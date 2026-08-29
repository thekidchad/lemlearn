// Package pdfsig appose une signature PAdES sur un PDF, par mise à jour
// incrémentale.
//
// Le principe est celui de la spécification PDF : les octets d'origine ne sont
// jamais réécrits. On leur ajoute une section contenant le dictionnaire de
// signature, le champ de formulaire correspondant, la page et le catalogue
// remis à jour, puis une nouvelle table xref chaînée à la précédente par
// /Prev. Un lecteur qui ouvre le fichier voit le document signé ; un
// vérificateur peut prouver que le contenu antérieur n'a pas bougé d'un octet.
//
// La signature couvre tout le fichier sauf la chaîne hexadécimale qui la
// contient — c'est le rôle de /ByteRange. Modifier quoi que ce soit d'autre,
// fût-ce un octet, invalide la signature de façon détectable dans n'importe
// quel lecteur conforme.
package pdfsig

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/lemlearn/api/internal/platform/cms"
)

// contentsCapacity dimensionne l'emplacement réservé à la signature CMS.
//
// Une signature RSA 2048 avec son certificat pèse environ 2 Kio ; une chaîne
// complète et un jeton d'horodatage la portent vers 8 Kio. La réserve est
// large à dessein : la place inutilisée est comblée de zéros, alors qu'une
// réserve trop courte oblige à tout recommencer — et le document a déjà été
// présenté au signataire.
const contentsCapacity = 16384

// Options décrit la signature à apposer.
type Options struct {
	Certificate *x509.Certificate
	PrivateKey  crypto.Signer
	Chain       []*x509.Certificate

	// Name, Reason et Location apparaissent dans le panneau de signature du
	// lecteur PDF.
	Name     string
	Reason   string
	Location string
	// FieldTitle nomme le champ de signature. C'est l'intitulé que montre le
	// panneau du lecteur : il doit désigner l'organisme de formation, pas
	// l'outil qui a produit le document.
	FieldTitle string

	SignedAt time.Time

	// Timestamper, s'il est fourni, fait passer la signature de PAdES-B-B à
	// PAdES-B-T en y incorporant un jeton RFC 3161.
	Timestamper func(signatureDigest []byte) ([]byte, error)
}

// Sign renvoie le PDF signé.
func Sign(pdf []byte, opts Options) ([]byte, error) {
	if opts.Certificate == nil || opts.PrivateKey == nil {
		return nil, fmt.Errorf("pdfsig: certificat ou clé manquant")
	}

	doc, err := parse(pdf)
	if err != nil {
		return nil, err
	}

	signedAt := opts.SignedAt
	if signedAt.IsZero() {
		signedAt = time.Now()
	}

	sigNum := doc.size
	fieldNum := doc.size + 1
	nextSize := doc.size + 2

	var out bytes.Buffer
	out.Write(pdf)
	// La section incrémentale commence sur une nouvelle ligne : coller les
	// octets à la suite de %%EOF empêcherait certains lecteurs de retrouver
	// la frontière entre les deux révisions.
	if !bytes.HasSuffix(pdf, []byte("\n")) {
		out.WriteByte('\n')
	}

	offsets := map[int]int{}

	offsets[sigNum] = out.Len()
	writeSignatureDict(&out, sigNum, opts, signedAt)

	offsets[fieldNum] = out.Len()
	fmt.Fprintf(&out, "%d 0 obj\n<<\n", fieldNum)
	out.WriteString("/Type /Annot\n/Subtype /Widget\n/FT /Sig\n")
	// Champ invisible : la représentation visuelle de la signature est déjà
	// dans le document, rendue par le gabarit dans sa zone. Superposer une
	// apparence de widget ferait double emploi et masquerait le tracé.
	out.WriteString("/Rect [0 0 0 0]\n/F 132\n")
	// Le nom du champ apparaît dans le panneau de signature du lecteur. Il
	// nomme l'organisme quand on le connaît : c'est lui qui délivre la
	// formation, et c'est son nom que cherche un financeur.
	champ := "Signature electronique"
	if opts.FieldTitle != "" {
		champ = opts.FieldTitle
	}
	fmt.Fprintf(&out, "/T %s\n/V %d 0 R\n/P %d 0 R\n", pdfString(champ), sigNum, doc.firstPage)
	out.WriteString(">>\nendobj\n")

	page, err := doc.objectWithAnnot(doc.firstPage, fieldNum)
	if err != nil {
		return nil, err
	}
	offsets[doc.firstPage] = out.Len()
	out.Write(page)

	catalog, err := doc.objectWithAcroForm(doc.root, fieldNum)
	if err != nil {
		return nil, err
	}
	offsets[doc.root] = out.Len()
	out.Write(catalog)

	xrefOffset := out.Len()
	writeXref(&out, offsets)
	writeTrailer(&out, doc, nextSize, xrefOffset)

	buf := out.Bytes()
	return seal(buf, opts, signedAt)
}

// writeSignatureDict écrit le dictionnaire de signature avec ses réserves.
//
// /ByteRange et /Contents sont écrits à leur taille définitive dès maintenant :
// les remplir plus tard ne doit déplacer aucun octet, sans quoi les décalages
// consignés dans /ByteRange deviendraient faux.
func writeSignatureDict(out *bytes.Buffer, num int, opts Options, signedAt time.Time) {
	fmt.Fprintf(out, "%d 0 obj\n<<\n", num)
	out.WriteString("/Type /Sig\n/Filter /Adobe.PPKLite\n/SubFilter /ETSI.CAdES.detached\n")
	if opts.Name != "" {
		fmt.Fprintf(out, "/Name %s\n", pdfString(opts.Name))
	}
	if opts.Reason != "" {
		fmt.Fprintf(out, "/Reason %s\n", pdfString(opts.Reason))
	}
	if opts.Location != "" {
		fmt.Fprintf(out, "/Location %s\n", pdfString(opts.Location))
	}
	fmt.Fprintf(out, "/M %s\n", pdfDate(signedAt))
	fmt.Fprintf(out, "/ByteRange [0 %010d %010d %010d]\n", 0, 0, 0)
	out.WriteString("/Contents <")
	out.Write(bytes.Repeat([]byte{'0'}, contentsCapacity*2))
	out.WriteString(">\n>>\nendobj\n")
}

var byteRangePattern = regexp.MustCompile(`/ByteRange \[0 (\d{10}) (\d{10}) (\d{10})\]`)

// seal remplit /ByteRange puis /Contents.
func seal(buf []byte, opts Options, signedAt time.Time) ([]byte, error) {
	start := bytes.LastIndex(buf, []byte("/Contents <"))
	if start < 0 {
		return nil, fmt.Errorf("pdfsig: emplacement de signature introuvable")
	}
	gapStart := start + len("/Contents ")
	gapEnd := bytes.IndexByte(buf[gapStart:], '>')
	if gapEnd < 0 {
		return nil, fmt.Errorf("pdfsig: emplacement de signature malformé")
	}
	gapEnd += gapStart + 1

	rangePos := byteRangePattern.FindIndex(buf)
	if rangePos == nil {
		return nil, fmt.Errorf("pdfsig: /ByteRange introuvable")
	}
	// Le remplacement doit faire exactement la même longueur que la réserve :
	// on complète par des espaces, que le format autorise entre les jetons.
	filled := fmt.Sprintf("/ByteRange [0 %d %d %d]", gapStart, gapEnd, len(buf)-gapEnd)
	if len(filled) > rangePos[1]-rangePos[0] {
		return nil, fmt.Errorf("pdfsig: /ByteRange trop long pour sa réserve")
	}
	copy(buf[rangePos[0]:rangePos[1]], append([]byte(filled),
		bytes.Repeat([]byte{' '}, rangePos[1]-rangePos[0]-len(filled))...))

	// Le contenu signé est le fichier entier moins la chaîne hexadécimale.
	signable := make([]byte, 0, len(buf)-(gapEnd-gapStart))
	signable = append(signable, buf[:gapStart]...)
	signable = append(signable, buf[gapEnd:]...)

	signature, err := cms.SignDetached(signable, cms.Options{
		Certificate: opts.Certificate,
		PrivateKey:  opts.PrivateKey,
		Chain:       opts.Chain,
		SigningTime: signedAt,
		Timestamper: opts.Timestamper,
	})
	if err != nil {
		return nil, err
	}
	if len(signature) > contentsCapacity {
		return nil, fmt.Errorf(
			"pdfsig: signature de %d octets, réserve de %d — augmentez contentsCapacity",
			len(signature), contentsCapacity)
	}

	encoded := make([]byte, contentsCapacity*2)
	for i := range encoded {
		encoded[i] = '0'
	}
	hex.Encode(encoded, signature)
	copy(buf[gapStart+1:gapEnd-1], encoded)

	return buf, nil
}

func writeXref(out *bytes.Buffer, offsets map[int]int) {
	nums := make([]int, 0, len(offsets))
	for num := range offsets {
		nums = append(nums, num)
	}
	// Les sous-sections d'une table xref doivent être ordonnées par numéro
	// d'objet croissant.
	for i := range nums {
		for j := i + 1; j < len(nums); j++ {
			if nums[j] < nums[i] {
				nums[i], nums[j] = nums[j], nums[i]
			}
		}
	}

	out.WriteString("xref\n")
	for i := 0; i < len(nums); {
		j := i
		for j+1 < len(nums) && nums[j+1] == nums[j]+1 {
			j++
		}
		fmt.Fprintf(out, "%d %d\n", nums[i], j-i+1)
		for k := i; k <= j; k++ {
			// Chaque entrée fait exactement vingt octets : la spécification
			// l'impose, et un lecteur les indexe par arithmétique.
			fmt.Fprintf(out, "%010d %05d n \n", offsets[nums[k]], 0)
		}
		i = j + 1
	}
}

func writeTrailer(out *bytes.Buffer, doc *document, size, xrefOffset int) {
	out.WriteString("trailer\n<<\n")
	fmt.Fprintf(out, "/Size %d\n/Root %d 0 R\n", size, doc.root)
	if doc.info > 0 {
		fmt.Fprintf(out, "/Info %d 0 R\n", doc.info)
	}
	if len(doc.id) > 0 {
		fmt.Fprintf(out, "/ID %s\n", doc.id)
	}
	// /Prev chaîne cette révision à la précédente : c'est ce qui permet à un
	// lecteur de reconstituer l'état antérieur et de constater qu'il est
	// intact.
	fmt.Fprintf(out, "/Prev %d\n>>\nstartxref\n%d\n%%%%EOF\n", doc.prevXref, xrefOffset)
}

func pdfString(value string) string {
	var b bytes.Buffer
	b.WriteByte('(')
	for _, r := range value {
		switch r {
		case '(', ')', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(')')
	return b.String()
}

// pdfDate rend une date au format PDF : D:AAAAMMJJHHmmSS+HH'mm'.
func pdfDate(at time.Time) string {
	local := at.UTC()
	return "(D:" + local.Format("20060102150405") + "+00'00')"
}

func atoi(value []byte) int {
	n, _ := strconv.Atoi(string(bytes.TrimSpace(value)))
	return n
}
