package automation

import (
	"context"
	"testing"

	pkgHttp "github.com/cortezaproject/corteza/server/pkg/http"
	"github.com/stretchr/testify/require"
)

func TestHttpRequestRestrictedNetworks(t *testing.T) {
	for _, target := range []string{
		"http://169.254.169.254/latest/meta-data/iam/security-credentials/",
		"http://[fd00:ec2::254]/latest/meta-data/",
		"http://100.100.100.200/latest/meta-data/",
		"http://[::ffff:169.254.169.254]/",
	} {
		t.Run(target, func(t *testing.T) {
			_, err := httpRequestHandler{}.send(context.Background(), &httpRequestSendArgs{Url: target, Method: "GET"})
			require.ErrorIs(t, err, pkgHttp.ErrRestrictedNetwork)
		})
	}
}
