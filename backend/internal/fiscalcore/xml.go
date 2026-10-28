package fiscalcore

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" // NF-e XMLDSig profile requires legacy SHA-1, not password hashing.
	"crypto/x509"
	"embed"
	"encoding/asn1"
	"encoding/base64"
	"encoding/xml"
	"io"
	"path"
	"strings"
	"sync"
	"time"

	"context"
	"github.com/beevik/etree"
	"github.com/lestrrat-go/helium"
	"github.com/lestrrat-go/helium/xsd"
	dsig "github.com/russellhaering/goxmldsig"
)

const NFeNS = "http://www.portalfiscal.inf.br/nfe"
const NFSeNS = "http://www.sped.fazenda.gov.br/nfse"
const MaxXML = 1 << 20

//go:embed schemas/*/*.xsd schemas/manifest.json
var schemaFiles embed.FS
var schemaCache sync.Map
var schemaMu sync.Mutex

func parseXML(raw []byte) (*etree.Document, error) {
	return parseXMLLimits(raw, MaxXML, 8192)
}

// Only the trusted SOAP journal envelope needs extra wrapper space/nodes.
// Extracted fiscal documents still go through the original strict bounds.
func parseXMLLimits(raw []byte, byteLimit, nodeLimit int) (*etree.Document, error) {
	if len(raw) == 0 || len(raw) > byteLimit {
		return nil, ErrXML
	}
	dec := xml.NewDecoder(bytes.NewReader(raw))
	depth, roots, nodes := 0, 0, 0
	ids := map[string]bool{}
	for {
		token, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, ErrXML
		}
		switch t := token.(type) {
		case xml.Directive:
			return nil, ErrXML
		case xml.ProcInst:
			if t.Target != "xml" || roots != 0 {
				return nil, ErrXML
			}
		case xml.StartElement:
			if depth == 0 {
				roots++
			}
			depth++
			nodes++
			if depth > 64 || nodes > nodeLimit || len(t.Attr) > 64 {
				return nil, ErrXML
			}
			attrs := map[xml.Name]bool{}
			for _, a := range t.Attr {
				if attrs[a.Name] {
					return nil, ErrXML
				}
				attrs[a.Name] = true
				if a.Name.Local == "Id" || a.Name.Local == "ID" || a.Name.Local == "id" {
					if ids[a.Value] || a.Value == "" {
						return nil, ErrXML
					}
					ids[a.Value] = true
				}
			}
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth == 0 && len(bytes.TrimSpace(t)) > 0 {
				return nil, ErrXML
			}
		}
	}
	if depth != 0 || roots != 1 {
		return nil, ErrXML
	}
	d := etree.NewDocument()
	if d.ReadFromBytes(raw) != nil || d.Root() == nil {
		return nil, ErrXML
	}
	return d, nil
}

func schemaPath(kind string) string {
	switch kind {
	case "nfe":
		return "schemas/nfe400/nfe_v4.00.xsd"
	case "nfe_authorization_response":
		return "schemas/nfe400/authorization_response_root.xsd"
	case "nfe_query":
		return "schemas/nfe400/consSitNFe_v4.00.xsd"
	case "nfe_query_response":
		return "schemas/nfe400/retConsSitNFe_v4.00.xsd"
	case "nfe_process":
		return "schemas/nfe400/procNFe_v4.00.xsd"
	case "nfe_invalidation":
		return "schemas/nfe400/inutNFe_v4.00.xsd"
	case "nfe_invalidation_response":
		return "schemas/nfe400/retInutNFe_v4.00.xsd"
	case "nfe_invalidation_process":
		return "schemas/nfe400/procInutNFe_v4.00.xsd"
	case "nfe_event":
		return "schemas/events100/evento_root.xsd"
	case "nfe_event_batch":
		return "schemas/events100/envEvento_v1.00.xsd"
	case "nfe_event_response":
		return "schemas/events100/retEnvEvento_v1.00.xsd"
	case "nfe_event_process":
		return "schemas/events100/procEventoNFe_v1.00.xsd"
	case "nfe_cancel_detail":
		return "schemas/events100/e110111_v1.00.xsd"
	case "nfse_dps":
		return "schemas/nfse101/DPS_v1.01.xsd"
	case "nfse_dps_homolog_20261020":
		return "schemas/nfseHomolog20261020/DPS_v1.01.xsd"
	case "nfse_document":
		return "schemas/nfse101/NFSe_v1.01.xsd"
	case "nfse_event":
		return "schemas/nfse101/pedRegEvento_v1.01.xsd"
	}
	return ""
}

// ValidateXML uses only embedded, hashed official schema files. It never follows
// a schemaLocation from the document, opens host files or fetches remote schemas.
func ValidateXML(kind string, raw []byte) error {
	byteLimit, nodeLimit := MaxXML, 8192
	if kind == "nfe_process" || kind == "nfe_event_process" || kind == "nfe_invalidation_process" {
		byteLimit, nodeLimit = 2*MaxXML, 8200
	}
	parsed, err := parseXMLLimits(raw, byteLimit, nodeLimit)
	if err != nil {
		return err
	}
	if kind == "nfse_dps_homolog_20261020" {
		environment := parsed.Root().FindElement("infDPS/tpAmb")
		if environment == nil || environment.NamespaceURI() != NFSeNS || environment.Text() != "2" {
			return ErrInput
		}
	}
	path := schemaPath(kind)
	if path == "" {
		return ErrUnsupported
	}
	value, ok := schemaCache.Load(kind)
	if !ok {
		schemaMu.Lock()
		value, ok = schemaCache.Load(kind)
		if !ok {
			compiled, err := compileSchema(kind)
			if err != nil {
				schemaMu.Unlock()
				return ErrSchema
			}
			value = compiled
			schemaCache.Store(kind, value)
		}
		schemaMu.Unlock()
	}
	doc, parseErr := helium.NewParser().Parse(context.Background(), raw)
	if parseErr != nil {
		return ErrXML
	}
	defer doc.Free()
	if xsd.NewValidator(value.(*xsd.Schema)).Validate(context.Background(), doc) != nil {
		return ErrSchema
	}
	return nil
}

type Credential struct {
	certificate   *x509.Certificate
	signer        crypto.Signer
	document      string
	chain         [][]byte
	verifiedChain []*x509.Certificate
}

// NewCredential verifies possession, validity and the e-CNPJ SAN binding. Trust
// roots must be supplied by trusted installation configuration, not HTTP input.
// Self-signed certificates are accepted only if explicitly trusted in that pool;
// tests use their own pool, never a production trust store.
func NewCredential(der []byte, key crypto.Signer, document string, roots *x509.CertPool, now time.Time) (*Credential, error) {
	return NewCredentialWithChain(der, nil, key, document, roots, now)
}

// NewCredentialWithChain accepts untrusted intermediate certificates separately
// from trusted roots. Intermediates are never promoted to trust anchors.
func NewCredentialWithChain(der []byte, intermediates [][]byte, key crypto.Signer, document string, roots *x509.CertPool, now time.Time) (*Credential, error) {
	cert, err := x509.ParseCertificate(der)
	if err != nil || key == nil || roots == nil || !docRE.MatchString(document) || cert.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return nil, ErrCredential
	}
	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok || pub.N.BitLen() < 2048 {
		return nil, ErrCredential
	}
	other, ok := key.Public().(*rsa.PublicKey)
	if !ok || pub.E != other.E || pub.N.Cmp(other.N) != 0 {
		return nil, ErrCredential
	}
	if len(intermediates) > 8 {
		return nil, ErrCredential
	}
	pool := x509.NewCertPool()
	for _, raw := range intermediates {
		intermediate, e := x509.ParseCertificate(raw)
		if e != nil {
			return nil, ErrCredential
		}
		pool.AddCert(intermediate)
	}
	chains, err := cert.Verify(x509.VerifyOptions{Roots: roots, Intermediates: pool, CurrentTime: now, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}})
	if err != nil {
		return nil, ErrCredential
	}
	if certificateCNPJ(cert) != document {
		return nil, ErrCredential
	}
	chain := [][]byte{append([]byte(nil), cert.Raw...)}
	for i, certificate := range chains[0] {
		if i > 0 && i < len(chains[0])-1 {
			chain = append(chain, append([]byte(nil), certificate.Raw...))
		}
	}
	return &Credential{certificate: cert, signer: key, document: document, chain: chain, verifiedChain: chains[0]}, nil
}

func certificateCNPJ(cert *x509.Certificate) string {
	var result string
	for _, e := range cert.Extensions {
		if !e.Id.Equal(asn1.ObjectIdentifier{2, 5, 29, 17}) {
			continue
		}
		var seq asn1.RawValue
		rest, err := asn1.Unmarshal(e.Value, &seq)
		if err != nil || len(rest) != 0 || seq.Tag != 16 {
			return ""
		}
		raw := seq.Bytes
		for len(raw) > 0 {
			var other asn1.RawValue
			raw, err = asn1.Unmarshal(raw, &other)
			if err != nil {
				return ""
			}
			if other.Class != 2 || other.Tag != 0 {
				continue
			}
			var oid asn1.ObjectIdentifier
			body, err := asn1.Unmarshal(other.Bytes, &oid)
			if err != nil {
				return ""
			}
			if !oid.Equal(asn1.ObjectIdentifier{2, 16, 76, 1, 3, 3}) {
				continue
			}
			var explicit, value asn1.RawValue
			tail, err := asn1.Unmarshal(body, &explicit)
			if err != nil || len(tail) != 0 || explicit.Class != 2 || explicit.Tag != 0 {
				return ""
			}
			tail, err = asn1.Unmarshal(explicit.Bytes, &value)
			if err != nil || len(tail) != 0 || (value.Tag != 12 && value.Tag != 19 && value.Tag != 22) {
				return ""
			}
			s := string(value.Bytes)
			if !docRE.MatchString(s) || (result != "" && result != s) {
				return ""
			}
			result = s
		}
	}
	return result
}

func payload(root *etree.Element, kind string) (*etree.Element, error) {
	rootTag, info, ns := "", "", ""
	switch kind {
	case "nfe":
		rootTag, info, ns = "NFe", "infNFe", NFeNS
	case "nfe_invalidation":
		rootTag, info, ns = "inutNFe", "infInut", NFeNS
	case "nfe_event":
		rootTag, info, ns = "evento", "infEvento", NFeNS
	case "nfse_dps", "nfse_dps_homolog_20261020":
		rootTag, info, ns = "DPS", "infDPS", NFSeNS
	case "nfse_event":
		rootTag, info, ns = "pedRegEvento", "infPedReg", NFSeNS
	default:
		return nil, ErrUnsupported
	}
	if root.Tag != rootTag || root.NamespaceURI() != ns {
		return nil, ErrXML
	}
	var found *etree.Element
	for _, e := range root.ChildElements() {
		if e.Tag == info && e.NamespaceURI() == ns {
			if found != nil {
				return nil, ErrXML
			}
			found = e
		} else if !(e.Tag == "Signature" && e.NamespaceURI() == dsig.Namespace) && !(kind == "nfe" && e.Tag == "infNFeSupl" && e.NamespaceURI() == ns) {
			return nil, ErrXML
		}
	}
	if found == nil || found.SelectAttrValue("Id", "") == "" {
		return nil, ErrXML
	}
	return found, nil
}

func (c *Credential) current(now time.Time) bool {
	if c == nil || c.certificate == nil || c.signer == nil || len(c.verifiedChain) == 0 {
		return false
	}
	for _, cert := range c.verifiedChain {
		if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
			return false
		}
	}
	return true
}

// SignXML signs the exact identified payload; no empty/external references are
// allowed. It verifies its own output before returning it. No key is serialized.
func (c *Credential) SignXML(kind string, raw []byte, now time.Time) ([]byte, error) {
	// Event signing for national NFS-e awaits author/mandate binding and tests.
	if kind == "nfse_event" {
		return nil, ErrUnsupported
	}
	if !c.current(now) {
		return nil, ErrCredential
	}
	// TNFe requires Signature: validate the complete schema after signing.
	if kind != "nfe" && kind != "nfe_event" && kind != "nfe_invalidation" {
		if err := ValidateXML(kind, raw); err != nil {
			return nil, err
		}
	}
	doc, err := parseXML(raw)
	if err != nil {
		return nil, err
	}
	info, err := payload(doc.Root(), kind)
	if err != nil {
		return nil, err
	}
	for _, el := range doc.Root().ChildElements() {
		if el.Tag == "Signature" {
			return nil, ErrXML
		}
	}
	// Issuer ownership is bound to the signed payload, never an outer wrapper.
	path := "emit/CNPJ"
	if kind == "nfse_dps" || kind == "nfse_dps_homolog_20261020" {
		path = "prest/CNPJ"
	}
	if kind == "nfe_event" || kind == "nfe_invalidation" {
		path = "CNPJ"
	}
	if kind != "nfse_event" {
		issuer := info.FindElement(path)
		if issuer == nil || issuer.Text() != c.document {
			return nil, ErrCredential
		}
	}
	ctx, err := dsig.NewSigningContext(c.signer, [][]byte{c.certificate.Raw})
	if err != nil {
		return nil, ErrCredential
	}
	ctx.Hash = crypto.SHA1
	ctx.IdAttribute = "Id"
	ctx.Prefix = ""
	ctx.Canonicalizer = dsig.MakeC14N10RecCanonicalizer()
	sig, err := ctx.ConstructSignature(info, true)
	if err != nil {
		return nil, ErrXML
	}
	doc.Root().AddChild(sig)
	signed, err := doc.WriteToBytes()
	if err != nil {
		return nil, ErrXML
	}
	if err = ValidateXML(kind, signed); err != nil {
		return nil, err
	}
	if err = c.VerifyXML(kind, signed, now); err != nil {
		return nil, err
	}
	return signed, nil
}

// VerifyXML uses the explicitly trusted issuer certificate and exact payload.
// It does not trust an embedded KeyInfo, duplicate ID, alternate signature,
// external URI, extra transform or arbitrary reference selected by an attacker.
func (c *Credential) VerifyXML(kind string, raw []byte, now time.Time) error {
	if kind == "nfse_event" {
		return ErrUnsupported
	}
	if !c.current(now) {
		return ErrCredential
	}
	return c.verifySignedXML(kind, raw)
}

// verifySignedXML checks integrity using an already selected public certificate.
// Live operations must use VerifyXML, which also requires current trusted
// credentials. Archive recovery selects its certificate only from the original
// trusted journal, never as a new source of authorization or certificate trust.
func (c *Credential) verifySignedXML(kind string, raw []byte) error {
	if err := ValidateXML(kind, raw); err != nil {
		return err
	}
	d, err := parseXML(raw)
	if err != nil {
		return err
	}
	info, err := payload(d.Root(), kind)
	if err != nil {
		return err
	}
	issuerPath := "emit/CNPJ"
	if kind == "nfse_dps" || kind == "nfse_dps_homolog_20261020" {
		issuerPath = "prest/CNPJ"
	}
	if kind == "nfe_event" || kind == "nfe_invalidation" {
		issuerPath = "CNPJ"
	}
	if kind != "nfse_event" {
		issuer := info.FindElement(issuerPath)
		if issuer == nil || issuer.Text() != c.document {
			return ErrCredential
		}
	}
	if kind == "nfe" {
		key := strings.TrimPrefix(info.SelectAttrValue("Id", ""), "NFe")
		if !ValidAccessKey(key) || key[6:20] != c.document {
			return ErrXML
		}
		if err = invoiceIdentity(info, key); err != nil {
			return err
		}
		if err = reconcileTaxTotals(info); err != nil {
			return err
		}
	}
	if kind == "nfe_event" {
		if err = cancellationIdentity(info, c.document); err != nil {
			return err
		}
	}
	if kind == "nfe_invalidation" {
		if _, err = invalidationInput(info); err != nil {
			return err
		}
	}

	var sig *etree.Element
	for _, e := range d.Root().ChildElements() {
		if e.Tag == "Signature" && e.NamespaceURI() == dsig.Namespace {
			if sig != nil {
				return ErrXML
			}
			sig = e
		}
	}
	if sig == nil {
		return ErrXML
	}
	si := sig.SelectElement("SignedInfo")
	if si == nil {
		return ErrXML
	}
	cm, sm, ref := si.SelectElement("CanonicalizationMethod"), si.SelectElement("SignatureMethod"), si.SelectElement("Reference")
	if cm == nil || sm == nil || ref == nil || len(si.ChildElements()) != 3 || cm.SelectAttrValue("Algorithm", "") != "http://www.w3.org/TR/2001/REC-xml-c14n-20010315" || sm.SelectAttrValue("Algorithm", "") != "http://www.w3.org/2000/09/xmldsig#rsa-sha1" || ref.SelectAttrValue("URI", "") != "#"+info.SelectAttrValue("Id", "") {
		return ErrXML
	}
	transforms := ref.SelectElement("Transforms")
	if transforms == nil || len(transforms.ChildElements()) != 2 {
		return ErrXML
	}
	for i, el := range transforms.ChildElements() {
		want := []string{"http://www.w3.org/2000/09/xmldsig#enveloped-signature", "http://www.w3.org/TR/2001/REC-xml-c14n-20010315"}[i]
		if el.Tag != "Transform" || el.SelectAttrValue("Algorithm", "") != want {
			return ErrXML
		}
	}
	dm, dv, sv, cert := ref.SelectElement("DigestMethod"), ref.SelectElement("DigestValue"), sig.SelectElement("SignatureValue"), sig.FindElement("KeyInfo/X509Data/X509Certificate")
	if dm == nil || dv == nil || sv == nil || cert == nil || dm.SelectAttrValue("Algorithm", "") != "http://www.w3.org/2000/09/xmldsig#sha1" {
		return ErrXML
	}
	for _, e := range []*etree.Element{si, cm, sm, ref, transforms, dm, dv, sv, cert} {
		if e.NamespaceURI() != dsig.Namespace {
			return ErrXML
		}
	}
	certBytes, err := base64.StdEncoding.DecodeString(cert.Text())
	if err != nil || !bytes.Equal(certBytes, c.certificate.Raw) {
		return ErrCredential
	}
	canonicalizer := dsig.MakeC14N10RecCanonicalizer()
	body, err := canonicalizer.Canonicalize(info)
	if err != nil {
		return ErrXML
	}
	sum := sha1.Sum(body)
	digest, err := base64.StdEncoding.DecodeString(dv.Text())
	if err != nil || !bytes.Equal(digest, sum[:]) {
		return ErrXML
	}
	body, err = canonicalizer.Canonicalize(si)
	if err != nil {
		return ErrXML
	}
	sum = sha1.Sum(body)
	signature, err := base64.StdEncoding.DecodeString(sv.Text())
	if err != nil {
		return ErrXML
	}
	if rsa.VerifyPKCS1v15(c.certificate.PublicKey.(*rsa.PublicKey), crypto.SHA1, sum[:], signature) != nil {
		return ErrXML
	}
	return nil
}

// signBytes is used by the documented NFC-e offline QR profile.
func (c *Credential) signBytes(raw []byte, now time.Time) ([]byte, error) {
	if !c.current(now) {
		return nil, ErrCredential
	}
	sum := sha1.Sum(raw)
	return c.signer.Sign(rand.Reader, sum[:], crypto.SHA1)
}

func compileSchema(kind string) (*xsd.Schema, error) {
	root := schemaPath(kind)
	if root == "" {
		return nil, ErrUnsupported
	}
	// Helium CompileFile opens its initial path through os.ReadFile even when
	// FS is set. Read the embedded root explicitly; FS resolves only its trusted
	// includes/imports. Installed executables must never depend on repository CWD.
	raw, err := schemaFiles.ReadFile(root)
	if err != nil {
		return nil, ErrSchema
	}
	doc, err := helium.NewParser().Parse(context.Background(), raw)
	if err != nil {
		return nil, ErrSchema
	}
	defer doc.Free()
	return xsd.NewCompiler().FS(schemaFiles).BaseDir(path.Dir(root)).Compile(context.Background(), doc)
}
