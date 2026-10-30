//go:build ignore

// 仅供本机自动验收生成临时证书，不进入发布程序。
package main
import("crypto/ecdsa";"crypto/elliptic";"crypto/rand";"crypto/x509";"encoding/json";"encoding/pem";"math/big";"net";"os";"time")
func main(){
 key,err:=ecdsa.GenerateKey(elliptic.P256(),rand.Reader);if err!=nil{panic(err)}
 tpl:=&x509.Certificate{SerialNumber:big.NewInt(1),DNSNames:[]string{"localhost"},IPAddresses:[]net.IP{net.ParseIP("127.0.0.1")},NotBefore:time.Now().Add(-time.Hour),NotAfter:time.Now().Add(24*time.Hour),KeyUsage:x509.KeyUsageDigitalSignature,ExtKeyUsage:[]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
 der,err:=x509.CreateCertificate(rand.Reader,tpl,tpl,&key.PublicKey,key);if err!=nil{panic(err)};private,err:=x509.MarshalECPrivateKey(key);if err!=nil{panic(err)}
 b,err:=json.Marshal(map[string]string{"domain":"localhost","pem":string(pem.EncodeToMemory(&pem.Block{Type:"CERTIFICATE",Bytes:der})),"key":string(pem.EncodeToMemory(&pem.Block{Type:"EC PRIVATE KEY",Bytes:private}))});if err!=nil{panic(err)}
 if err=os.WriteFile(os.Args[1],b,0600);err!=nil{panic(err)}
}
