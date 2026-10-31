// Package fiscalcert creates disposable identities for automated tests only.
// No static certificate/private key or production trust anchor is distributed.
package fiscalcert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"time"

	"titansystem-backend/internal/fiscalcore"
)

type Fixture struct {
	Root       *x509.Certificate
	RootKey    *rsa.PrivateKey
	Leaf       *x509.Certificate
	LeafKey    *rsa.PrivateKey
	Credential *fiscalcore.Credential
	Roots      *x509.CertPool
	Server     tls.Certificate
}

func New(document, host string) (*Fixture, error) {
	f := &Fixture{}
	var err error
	f.RootKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	root := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Synthetic fiscal test CA only"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(2 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign, SubjectKeyId: []byte{1, 2, 3, 4}}
	der, err := x509.CreateCertificate(rand.Reader, root, root, &f.RootKey.PublicKey, f.RootKey)
	if err != nil {
		return nil, err
	}
	f.Root, err = x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	f.Roots = x509.NewCertPool()
	f.Roots.AddCert(f.Root)
	f.LeafKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	oid, _ := asn1.Marshal(asn1.ObjectIdentifier{2, 16, 76, 1, 3, 3})
	value, _ := asn1.Marshal(document)
	explicit, _ := asn1.Marshal(asn1.RawValue{Class: 2, Tag: 0, IsCompound: true, Bytes: value})
	san, _ := asn1.Marshal([]asn1.RawValue{{Class: 2, Tag: 0, IsCompound: true, Bytes: append(oid, explicit...)}})
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "Synthetic issuer only"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, ExtraExtensions: []pkix.Extension{{Id: asn1.ObjectIdentifier{2, 5, 29, 17}, Value: san}}}
	der, err = x509.CreateCertificate(rand.Reader, leaf, f.Root, &f.LeafKey.PublicKey, f.RootKey)
	if err != nil {
		return nil, err
	}
	f.Leaf, err = x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	f.Credential, err = fiscalcore.NewCredential(der, f.LeafKey, document, f.Roots, now)
	if err != nil {
		return nil, err
	}
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	server := &x509.Certificate{SerialNumber: big.NewInt(3), Subject: pkix.Name{CommonName: host}, DNSNames: []string{host}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err = x509.CreateCertificate(rand.Reader, server, f.Root, &serverKey.PublicKey, f.RootKey)
	if err != nil {
		return nil, err
	}
	f.Server = tls.Certificate{Certificate: [][]byte{der}, PrivateKey: serverKey}
	return f, nil
}
func (f *Fixture) CRL(revoked bool, from, until time.Time) ([]byte, error) {
	list := &x509.RevocationList{Number: big.NewInt(1), ThisUpdate: from, NextUpdate: until}
	if revoked {
		list.RevokedCertificateEntries = []x509.RevocationListEntry{{SerialNumber: f.Leaf.SerialNumber, RevocationTime: from}}
	}
	return x509.CreateRevocationList(rand.Reader, list, f.Root, f.RootKey)
}
