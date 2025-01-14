package basic

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"io/ioutil"
	"math/big"

	"github.com/hyperledger/fabric/bccsp/utils"
	"github.com/hyperledger/fabric/common/crypto"
	"github.com/hyperledger/fabric/protos/common"
	"github.com/pkg/errors"
)

type CryptoConfig struct {
	MSPID      string   `json:"name"`
	PrivKey    string   `json:"private_key"`
	SignCert   string   `json:"sign_cert"`
	TLSCACerts []string `json:"tls_ca_cert"`
}

type ECDSASignature struct {
	R, S *big.Int
}

type Crypto struct {
	Creator    []byte
	PrivKey    *ecdsa.PrivateKey
	SignCert   *x509.Certificate
	TLSCACerts [][]byte
}

func (s *Crypto) Sign(message []byte) ([]byte, error) {
	ri, si, err := ecdsa.Sign(rand.Reader, s.PrivKey, digest(message))
	if err != nil {
		return nil, err
	}

	si, _, err = utils.ToLowS(&s.PrivKey.PublicKey, si)
	if err != nil {
		return nil, err
	}

	return asn1.Marshal(ECDSASignature{ri, si})
}

func (s *Crypto) Serialize() ([]byte, error) {
	return s.Creator, nil
}

func (s *Crypto) NewSignatureHeader() (*common.SignatureHeader, error) {
	creator, err := s.Serialize()
	if err != nil {
		return nil, err
	}
	nonce, err := crypto.GetRandomNonce()
	if err != nil {
		return nil, err
	}

	return &common.SignatureHeader{
		Creator: creator,
		Nonce:   nonce,
	}, nil
}

func GetTLSCACerts(files []string) ([][]byte, error) {
	var certs [][]byte
	for _, f := range files {
		in, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, err
		}

		certs = append(certs, in)
	}

	return certs, nil
}

func digest(in []byte) []byte {
	h := sha256.New()
	h.Write(in)
	return h.Sum(nil)
}

func toPEM(in []byte) ([]byte, error) {
	d := make([]byte, base64.StdEncoding.DecodedLen(len(in)))
	n, err := base64.StdEncoding.Decode(d, in)
	if err != nil {
		return nil, err
	}
	return d[:n], nil
}

func GetPrivateKey(f string) (*ecdsa.PrivateKey, error) {
	in, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, err
	}

	k, err := utils.PEMtoPrivateKey(in, []byte{})
	if err != nil {
		return nil, err
	}

	key, ok := k.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.Errorf("expecting ecdsa key")
	}

	return key, nil
}

func GetCertificate(f string) (*x509.Certificate, []byte, error) {
	in, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, nil, err
	}

	block, _ := pem.Decode(in)

	c, err := x509.ParseCertificate(block.Bytes)
	return c, in, err
}
