package config

import (
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type TunnelTestSuite struct {
	suite.Suite
}

func (s *TunnelTestSuite) TestDefaultOff() {
	httpProxy := "http://127.0.0.1:1080"
	tunnel := parseTunnel(httpProxy, httpProxy, "*:false,x.y:true,j.k:http://warp:1080")

	proxy := tunnel.getProxy("*")
	s.NotNil(proxy)
	s.Equal(proxy.String(), httpProxy)

	s.Equal(os.Getenv("NO_PROXY"), "*")

	s.Nil(tunnel.getProxy("abc.xyz"))
	proxy, err := tunnel.forcedProxy(&http.Request{
		URL: &url.URL{Host: "abc.xyz", Scheme: "https"},
	})
	s.Nil(err)
	s.NotNil(proxy)
	s.Equal(proxy.String(), httpProxy)

	proxy, err = tunnel.forcedProxy(&http.Request{
		URL: &url.URL{Host: "abc.j.k", Scheme: "https"},
	})
	s.Nil(err)
	s.NotNil(proxy)
	s.Equal(proxy.String(), "http://warp:1080")

	s.Nil(tunnel.getProxy("x.y.z"))

	s.Nil(tunnel.getProxy("xy.z"))

	proxy = tunnel.getProxy("x.y")
	s.NotNil(proxy)
	s.Equal(proxy.String(), httpProxy)

	proxy = tunnel.getProxy("a.x.y")
	s.NotNil(proxy)
	s.Equal(proxy.String(), httpProxy)
}

func (s *TunnelTestSuite) TestDefaultOn() {
	httpProxy := "http://127.0.0.1:1080"
	tunnel := parseTunnel(httpProxy, httpProxy, "*:true,x.y:false,j.k:http://warp:1080")

	proxy := tunnel.getProxy("*")
	s.NotNil(proxy)
	s.Equal(proxy.String(), httpProxy)

	s.Equal(os.Getenv("NO_PROXY"), "")

	s.Nil(tunnel.getProxy("abc.xyz"))
	proxy, err := tunnel.autoProxy(&http.Request{
		URL: &url.URL{Host: "abc.xyz", Scheme: "https"},
	})
	s.Nil(err)
	s.NotNil(proxy)
	s.Equal(proxy.String(), httpProxy)

	proxy, err = tunnel.autoProxy(&http.Request{
		URL: &url.URL{Host: "x.y.z", Scheme: "https"},
	})
	s.Nil(err)
	s.NotNil(proxy)
	s.Equal(proxy.String(), httpProxy)

	proxy, err = tunnel.autoProxy(&http.Request{
		URL: &url.URL{Host: "j.k", Scheme: "https"},
	})
	s.Nil(err)
	s.NotNil(proxy)
	s.Equal(proxy.String(), "http://warp:1080")

	s.Equal(tunnel.getProxy("x.y"), &url.URL{})

	s.Equal(tunnel.getProxy("a.x.y"), &url.URL{})
}

func (s *TunnelTestSuite) TestNewzNzbGrabProxy() {
	httpProxy := "http://127.0.0.1:1080"
	grabProxy := "http://grab:1080"
	tunnel := parseTunnel(httpProxy, httpProxy, "*:true,nzb.example:false,host.example:http://warp:1080,[newz_nzb_grab]:"+grabProxy)

	proxy, err := tunnel.newzNzbGrabProxy(&http.Request{
		URL: &url.URL{Host: "host.example", Scheme: "https"},
	})
	s.Nil(err)
	s.NotNil(proxy)
	s.Equal(proxy.String(), "http://warp:1080")

	proxy, err = tunnel.newzNzbGrabProxy(&http.Request{
		URL: &url.URL{Host: "nzb.example", Scheme: "https"},
	})
	s.Nil(err)
	s.Nil(proxy)

	proxy, err = tunnel.newzNzbGrabProxy(&http.Request{
		URL: &url.URL{Host: "deep.host.example", Scheme: "https"},
	})
	s.Nil(err)
	s.NotNil(proxy)
	s.Equal(proxy.String(), "http://warp:1080")

	proxy, err = tunnel.newzNzbGrabProxy(&http.Request{
		URL: &url.URL{Host: "unknown.example", Scheme: "https"},
	})
	s.Nil(err)
	s.NotNil(proxy)
	s.Equal(proxy.String(), grabProxy)

	bareTunnel := parseTunnel("", "", "")
	proxy, err = bareTunnel.newzNzbGrabProxy(&http.Request{
		URL: &url.URL{Host: "unknown.example", Scheme: "https"},
	})
	s.Nil(err)
	s.Nil(proxy)
}

func TestTunnel(t *testing.T) {
	suite.Run(t, new(TunnelTestSuite))
}
