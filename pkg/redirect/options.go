package redirect

import (
	"github.com/spf13/pflag"
)

type Options struct {
	CertDir    string
	CertEmail  string
	Hosts      []string
	Port       int
	EnableSSL  bool
	OrgMapping map[string]string
}

func NewOptions() *Options {
	return &Options{
		CertDir:   "certs",
		CertEmail: "tamal@appscode.com",
		Hosts:     []string{"r.appscode.com", "r.appscode.ninja", "r.byte.builders"},
		Port:      8080,
		OrgMapping: map[string]string{
			"appscode":  "appscode",
			"charts":    "appscode-charts",
			"kubedb":    "kubedb",
			"kubeform":  "kubeform",
			"kubestash": "kubestash",
			"kubevault": "kubevault",
			"library":   "appscode-images",
			"stash":     "stashed",
			"voyager":   "voyagermesh",
		},
	}
}

func (s *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.CertDir, "ssl.cert-dir", s.CertDir, "Directory where certs are stored")
	fs.StringVar(&s.CertEmail, "ssl.email", s.CertEmail, "Email used by Let's Encrypt to notify about problems with issued certificates")
	fs.StringSliceVar(&s.Hosts, "ssl.hosts", s.Hosts, "Hosts for which certificate will be issued")
	fs.IntVar(&s.Port, "port", s.Port, "Port used when SSL is not enabled")
	fs.BoolVar(&s.EnableSSL, "ssl", s.EnableSSL, "Set true to enable SSL via Let's Encrypt")
	fs.StringToStringVar(&s.OrgMapping, "org-mapping", s.OrgMapping, "Proxy org to GitHub org mappings")
}
