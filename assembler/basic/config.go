package basic

import (
	"encoding/json"
	"github.com/hcg1314/stupid/assembler"
	"io/ioutil"

	"github.com/gogo/protobuf/proto"
	"github.com/hyperledger/fabric/protos/msp"
)

type Node struct {
	Addr         string `json:"addr"`
	OverrideName string `json:"override_name"`
}

type Config struct {
	Peers         []Node   `json:"peers"`
	Orderer       Node     `json:"orderer"`
	Channel       string   `json:"channel"`
	Chaincode     string   `json:"chaincode"`
	MSPID         string   `json:"mspid"`
	PrivateKey    string   `json:"private_key"`
	SignCert      string   `json:"sign_cert"`
	TLSCACerts    []string `json:"tls_ca_certs"`
	NumOfConn     int      `json:"num_of_conn"`
	ClientPerConn int      `json:"client_per_conn"`
}

func LoadConfig(f string) *Config {
	raw, err := ioutil.ReadFile(f)
	if err != nil {
		panic(err)
	}

	config := &Config{}
	if err = json.Unmarshal(raw, config); err != nil {
		panic(err)
	}

	return config
}

func (c Config) LoadCrypto() *Crypto {
	conf := CryptoConfig{
		assembler.MSPID:      c.MSPID,
		assembler.PrivKey:    c.PrivateKey,
		assembler.SignCert:   c.SignCert,
		assembler.TLSCACerts: c.TLSCACerts,
	}

	priv, err := GetPrivateKey(assembler.PrivKey)
	if err != nil {
		panic(err)
	}

	cert, certBytes, err := GetCertificate(assembler.SignCert)
	if err != nil {
		panic(err)
	}

	id := &msp.SerializedIdentity{
		Mspid:   assembler.MSPID,
		IdBytes: certBytes,
	}

	name, err := proto.Marshal(id)
	if err != nil {
		panic(err)
	}

	certs, err := GetTLSCACerts(assembler.TLSCACerts)
	if err != nil {
		panic(err)
	}

	return &Crypto{
		assembler.Creator:    name,
		assembler.PrivKey:    priv,
		assembler.SignCert:   cert,
		assembler.TLSCACerts: certs,
	}
}
