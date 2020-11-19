package prometheus

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"

	"emperror.dev/errors"
	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/common/log"

	// . "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type PrometheusSecureClientConfig struct {
	Address string

	Token string

	UserAuth *UserAuth

	ServerCertFile string

	CaCert *[]byte
}

type UserAuth struct {
	Username, Password string
}

func ProvideApiClientFromCert(
	apiTokenPath string,
	promService *corev1.Service,
	caCert *[]byte,
	auth string,
) (api.Client, error) {

	var port int32
	if promService == nil {
		return nil, errors.New("Prometheus service not defined")
	}

	name := promService.Name
	namespace := promService.Namespace
	targetPort := intstr.FromString("rbac")
	
	switch {
	case targetPort.Type == intstr.Int:
		port = targetPort.IntVal
	default:
		for _, p := range promService.Spec.Ports {
			if p.Name == targetPort.StrVal {
				port = p.Port
			}
		}
	}

	conf, err := NewSecureClientFromCert(&PrometheusSecureClientConfig{
		Address:        fmt.Sprintf("https://%s.%s.svc:%v", name, namespace, port),
		Token:          auth,
		CaCert: caCert,
	})

	if err != nil {
		log.Error(err,"failed to setup NewSecureClient")
		return nil, err
	}


	if conf == nil {
		return nil,errors.New("client configuration is nil")
	}

	return conf, nil
}

func GetAuthToken(apiTokenPath string)(token string, returnErr error){
	content, err := ioutil.ReadFile(apiTokenPath)
	if err != nil {
		return "", err
	}
	token = fmt.Sprintf(string(content))
	return token, nil
}

func NewSecureClientFromCert(config *PrometheusSecureClientConfig) (api.Client, error) {
	tlsConfig, err := generateCACertPoolFromCert(*config.CaCert)
	fmt.Println("TLSCONFIG :\n",tlsConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tlsConfig")
	}

	var transport http.RoundTripper

	transport = &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	if config.UserAuth != nil {
		transport = WithBasicAuth(transport, config.UserAuth.Username, config.UserAuth.Password)
	}

	if config.Token != "" {
		transport = WithBearerAuth(transport, config.Token)
	}

	client, err := api.NewClient(api.Config{
		Address:      config.Address,
		RoundTripper: transport,
	})

	return client, err
}

func generateCACertPoolFromCert(caCert []byte) (*tls.Config, error) {
	caCertPool, err := x509.SystemCertPool()

	if err != nil {
		return nil, errors.Wrap(err, "failed to get system cert pool")
	}

	caCertPool.AppendCertsFromPEM(caCert)

	return &tls.Config{
		RootCAs: caCertPool,
	}, nil
}

type withHeader struct {
	http.Header
	rt http.RoundTripper
}

func WithBasicAuth(rt http.RoundTripper, username, password string) http.RoundTripper {
	addHead := WithHeader(rt)
	addHead.Header.Set("Authorization", "Basic "+basicAuth(username, password))
	return addHead
}

func WithBearerAuth(rt http.RoundTripper, token string) http.RoundTripper {
	addHead := WithHeader(rt)
	addHead.Header.Set("Authorization", "Bearer "+token)
	return addHead
}

func WithHeader(rt http.RoundTripper) withHeader {
	if rt == nil {
		rt = http.DefaultTransport
	}

	return withHeader{Header: make(http.Header), rt: rt}
}

func (h withHeader) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range h.Header {
		req.Header[k] = v
	}

	return h.rt.RoundTrip(req)
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}