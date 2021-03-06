# NewCertsPhase 平面分析
###### 分析版本为1.22，代码入口 cmd/kubeadm/app/cmd/init.go
##### NewCertsPhase 平面主要工作是生成证书

##### 1、Certs 平面涉及子平面，runCerts 函数负责 Print 提示，主要创建任务还是在子平面里
```
//newCertSubPhases() 为子平面, kubeadm init phase certs all 会先执行 runCerts，runCerts 执行完毕再执行子平面等到所有函数处理结束返回结构体
return workflow.Phase{
	Name:   "certs",
	Short:  "Certificate generation",
	Phases: newCertSubPhases(),
	Run:    runCerts,
	Long:   cmdutil.MacroCommandLongDescription,
}
```

##### 2、runCerts 函数会 print 一些数据作为生成执行证书的提示，输出后会检测初始化有没有外部 ca 的情况，如果有外部 ca，生成证书这个平面不会执行
```
fmt.Printf("[certs] Using certificateDir folder %q\n", data.CertificateWriteDir())

// If using an external CA while dryrun, copy CA cert to dryrun dir for later use
if data.ExternalCA() && data.DryRun() {
		externalCAFile := filepath.Join(data.Cfg().CertificatesDir, kubeadmconstants.CACertName)
		fileInfo, _ := os.Stat(externalCAFile)
		contents, err := os.ReadFile(externalCAFile)
		if err != nil {
			return err
		}
		err = os.WriteFile(filepath.Join(data.CertificateWriteDir(), kubeadmconstants.CACertName), contents, fileInfo.Mode())
		if err != nil {
			return err
		}
	}
```

##### 3、newCertSubPhases() 子平面作为主要执行创建 ca 的任务
```
//这边定一个子平面的结构体，通过 append 追加各个证书结构体到 subPhases 结构体中，最后执行 range 循环执行结构体
subPhases := []workflow.Phase{}

//RunAllSiblings 这个为true，代表着执行相同等级的所有平面
allPhase := workflow.Phase{
		Name:           "all",
		Short:          "Generate all certificates",
		InheritFlags:   getCertPhaseFlags("all"),
		RunAllSiblings: true,
	}

subPhases = append(subPhases, allPhase)   

//certsphase.GetDefaultCertList() 存放了 ca 的结构体函数
for _, cert := range certsphase.GetDefaultCertList() {
		var phase workflow.Phase
        //结构体里有CAName这个数据，如果结构体 CAName 为 "", 那么定义这个平面的 run 为 runCAPhase(cert), 
		//如果有定义平面的 run 为 runCertPhase(cert, lastCACert), range 循环 GetDefaultCertList 是有顺序的，
		//第一个结构体为 ca 结构体，ca 结构体 ca.Name 为 "",所以会执行 runCAPhase(cert) 函数生成ca文件，
		//之后 apiserver 的 runCertPhase 函数会用到 lastCaert 结构体生成 apiserver 证书
		if cert.CAName == "" {
			phase = newCertSubPhase(cert, runCAPhase(cert))
			lastCACert = cert
		} else {
			phase = newCertSubPhase(cert, runCertPhase(cert, lastCACert))
		}
		subPhases = append(subPhases, phase)
	}   

func GetDefaultCertList() Certificates {
	return Certificates{
		KubeadmCertRootCA(),
		KubeadmCertAPIServer(),
		KubeadmCertKubeletClient(),
		// Front Proxy certs
		KubeadmCertFrontProxyCA(),
		KubeadmCertFrontProxyClient(),
		// etcd certs
		KubeadmCertEtcdCA(),
		KubeadmCertEtcdServer(),
		KubeadmCertEtcdPeer(),
		KubeadmCertEtcdHealthcheck(),
		KubeadmCertEtcdAPIClient(),
	}
}

// KubeadmCertRootCA is the definition of the Kubernetes Root CA for the API Server and kubelet.
func KubeadmCertRootCA() *KubeadmCert {
	return &KubeadmCert{
		Name:     "ca",
		LongName: "self-signed Kubernetes CA to provision identities for other Kubernetes components",
		BaseName: kubeadmconstants.CACertAndKeyBaseName,
		config: pkiutil.CertConfig{
			Config: certutil.Config{
				CommonName: "kubernetes",
			},
		},
	}
}

// KubeadmCertAPIServer is the definition of the cert used to serve the Kubernetes API.
func KubeadmCertAPIServer() *KubeadmCert {
	return &KubeadmCert{
		Name:     "apiserver",
		LongName: "certificate for serving the Kubernetes API",
		BaseName: kubeadmconstants.APIServerCertAndKeyBaseName,
		CAName:   "ca",
		config: pkiutil.CertConfig{
			Config: certutil.Config{
				CommonName: kubeadmconstants.APIServerCertCommonName,
				Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			},
		},
		configMutators: []configMutatorsFunc{
			makeAltNamesMutator(pkiutil.GetAPIServerAltNames),
		},
	}
}

//除了上面的 range 循环获取的平面后，最后还定义了一个 sa 平面，每个平面都会生成 crt、key，sa 平面会生成 pub、key
//所有平面如下：
// "ca"
// "apiserver"
// "apiserver-kubelet-client"
// "front-proxy-ca"
// "front-proxy-client"
// "etcd-ca"
// "etcd-server"
// "etcd-peer"
// "etcd-healthcheck-client"
// "apiserver-etcd-client"
// "sa"

// SA creates the private/public key pair, which doesn't use x509 at all
	saPhase := workflow.Phase{
		Name:         "sa",
		Short:        "Generate a private key for signing service account tokens along with its public key",
		Long:         saKeyLongDesc,
		Run:          runCertsSa,
		InheritFlags: []string{options.CertificatesDir},
	}
```

##### 4、runCAPhase(cert) 函数把证书的结构体传进来处理
```
// 如果初始化有外部 etcd 与 ca.Name 等于 etcd-ca 则直接返回
// if using external etcd, skips etcd certificate authority generation
		if data.Cfg().Etcd.External != nil && ca.Name == "etcd-ca" {
			fmt.Printf("[certs] External etcd mode: Skipping %s certificate authority generation\n", ca.BaseName)
			return nil
		}

//这边加载 ca.BaseName，这个时候如果你有证书文件 crt 存在并且有内容则会报错，直接 fmt.Printf 返回 nil，
//如果没有 crt 文件存在，再检查 key 文件是否存在。
if cert, err := pkiutil.TryLoadCertFromDisk(data.CertificateDir(), ca.BaseName); err == nil {
			certsphase.CheckCertificatePeriodValidity(ca.BaseName, cert)

			if _, err := pkiutil.TryLoadKeyFromDisk(data.CertificateDir(), ca.BaseName); err == nil {
				fmt.Printf("[certs] Using existing %s certificate authority\n", ca.BaseName)
				return nil
			}
			fmt.Printf("[certs] Using existing %s keyless certificate authority\n", ca.BaseName)
			return nil
		}  

// TryLoadCertFromDisk 函数会检测你本地是否存在 crt 文件是否有内容，并且内容是否正确
func TryLoadCertFromDisk(pkiPath, name string) (*x509.Certificate, error) {
    //pathForCert主要是 join pkiPaht，name 连接字符串用的
	certificatePath := pathForCert(pkiPath, name)

    //CertsFromFile 判断 certs 是否正确
	certs, err := certutil.CertsFromFile(certificatePath)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't load the certificate file %s", certificatePath)
	}

	// We are only putting one certificate in the certificate pem file, so it's safe to just pick the first one
	// TODO: Support multiple certs here in order to be able to rotate certs
	cert := certs[0]

	return cert, nil
}

//所有检测都过了，没有外部 etcd，没有外部 ca，没有 crt、key 文件，则创建新的证书文件
return certsphase.CreateCACertAndKeyFiles(ca, cfg)

//CreateCACertAndKeyFiles 函数主要为创建这些 caCert 数据并返回结构体，
caCert, caKey, err := pkiutil.NewCertificateAuthority(certConfig)
	if err != nil {
		return err
	}

return writeCertificateAuthorityFilesIfNotExist(
		cfg.CertificatesDir,
		certSpec.BaseName,
		caCert,
		caKey,
	)

// NewSelfSignedCACert creates a CA certificate
func NewSelfSignedCACert(cfg Config, key crypto.Signer) (*x509.Certificate, error) {
	now := time.Now()
	tmpl := x509.Certificate{
		SerialNumber: new(big.Int).SetInt64(0),
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: cfg.Organization,
		},
		DNSNames:              []string{cfg.CommonName},
		NotBefore:             now.UTC(),
		NotAfter:              now.Add(duration365d * 10).UTC(),  //此版本证书已经为10年了
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDERBytes, err := x509.CreateCertificate(cryptorand.Reader, &tmpl, &tmpl, key.Public(), key)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(certDERBytes)
}

先格式化 key 内容通过 x509
// MarshalPrivateKeyToPEM converts a known private key type of RSA or ECDSA to
// a PEM encoded block or returns an error.
func MarshalPrivateKeyToPEM(privateKey crypto.PrivateKey) ([]byte, error) {
	switch t := privateKey.(type) {
	case *ecdsa.PrivateKey:
		derBytes, err := x509.MarshalECPrivateKey(t)
		if err != nil {
			return nil, err
		}
		block := &pem.Block{
			Type:  ECPrivateKeyBlockType,
			Bytes: derBytes,
		}
		return pem.EncodeToMemory(block), nil
	case *rsa.PrivateKey:
		block := &pem.Block{
			Type:  RSAPrivateKeyBlockType,
			Bytes: x509.MarshalPKCS1PrivateKey(t),
		}
		return pem.EncodeToMemory(block), nil
	default:
		return nil, fmt.Errorf("private key is not a recognized type: %T", privateKey)
	}
}

//这边是先创 key 再创 crt
if err := pkiutil.WriteCertAndKey(pkiDir, baseName, caCert, caKey); err != nil {
			return errors.Wrapf(err, "failure while saving %s certificate and key", baseName)
		}

//最终先创建文件给 0755 权限，在写入文件给 0600 权限
func WriteKey(keyPath string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(keyPath), os.FileMode(0755)); err != nil {
		return err
	}
	return ioutil.WriteFile(keyPath, data, os.FileMode(0600))
}    
```