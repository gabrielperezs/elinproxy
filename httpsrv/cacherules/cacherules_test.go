package cacherules

import (
	"bufio"
	"bytes"
	"log"
	"net/http"
	"testing"
)

func TestCacheRulesResponse(t *testing.T) {
	rawReq := []byte(`GET /wp-content/themes/longsocial/assets/fonts/fontawesome-webfont.woff2?v=4.4.0 HTTP/1.1
Host: example.com
Connection: keep-alive
Pragma: no-cache
Cache-Control: no-cache
Origin: http://example.com
User-Agent: Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/71.0.3578.98 Safari/537.36
Accept: */*
Referer: http://example.com/wp-content/themes/longsocial/assets/css/app.css?ver=1.0
Accept-Encoding: gzip, deflate
Accept-Language: es-ES,es;q=0.9,en-GB;q=0.8,en;q=0.7,ca-ES;q=0.6,ca;q=0.5
Cookie: _ga=GA1.2.1781578831.1543794210; __gads=ID=a31d2c01977040f4:T=1543794211:S=ALNI_Mb8efcLSBkgOl6RsA2vBdTkGQ25CA; _pubcid=c8913c3a-6c6b-428c-abff-5c62ea21abe4; sas_euconsent=BOYLE2ROYLE2RAKALBENB7-AAAAix7_______9______9uz_Gv_v_f__33e8__9v_l_7_-___u_-33d4-_1vf99yfm1-7ftr3tp_87ues2_Xur_959__3z3_NIA; __qca=P0-1003973449-1543794211302; advanced_ads_pro_visitor_referrer=http%3A//example.com/; _gid=GA1.2.174772353.1545570513; _cmpQcif3pcsupported=1; PHPSESSID=f8u7f2a6fpiqjentehsbv80e27; cookie_notice_accepted=true; advanced_ads_browser_width=1709; _gat=1; GED_PLAYLIST_ACTIVITY=W3sidSI6Ijd2RUgiLCJ0c2wiOjE1NDU5MDcwMzcsIm52IjowLCJ1cHQiOjE1NDU5MDcwMzEsImx0IjoxNTQ1OTA3MDMxfV0.; advanced_ads_page_impressions=94

	`)

	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(rawReq)))
	if err != nil {
		t.Fatalf("Read: %s", err)
	}
	rawResp := []byte(`HTTP/1.1 200 OK
Accept-Ranges: bytes
Age: 859
Cache-Control: max-age=2592000
Content-Length: 64464
Date: Thu, 27 Dec 2018 10:37:17 GMT
Etag: "fbd0-525b2a92c3a33"
Expires: Sat, 26 Jan 2019 10:22:58 GMT
Last-Modified: Sun, 29 Nov 2015 19:09:16 GMT
Server: nginx/1.10.3 (Ubuntu)
X-Boxqos-Device: pc
X-Cache: HIT
X-Rate-Limit-Duration: 1
X-Rate-Limit-Limit: 100.00
X-Rate-Limit-Request-Forwarded-For: 
X-Rate-Limit-Request-Remote-Addr: 37.15.5.115:50858
Content-Type: font/woff2

	`)

	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(rawResp)), req)
	if err != nil {
		t.Fatalf("Read response: %s", err)
	}

	rules := InternalRules{
		NoReqHeaders: map[string]string{
			"Cache-Control": "private",
		},
		NoReqCookieContains: []string{
			"wp-postpass_",
			"wordpress_logged_in",
			"woocommerce_cart_hash",
			"woocommerce_items_in_cart",
			"wp_woocommerce_session",
		},
		RespContentTypeTTLString: map[string]string{
			"font/": "1h",
		},
		RespStatusCodeTTLString: map[string]string{
			"200": "1m",
		},
	}
	if err := rules.Parse(); err != nil {
		t.Errorf("Parse: %s", err)
	}

	log.Printf("RespStatusCodeTTL: %+v", rules.RespStatusCodeTTL)
	log.Printf("RespContentTypeTTL: %+v", rules.RespContentTypeTTL)

	if ok := rules.IsReqCachable(req); !ok {
		t.Errorf("Request: %+v ==> %v", req, ok)
	}

	if ttl, ok := rules.IsRespCachable(resp); !ok {
		t.Errorf("Response: (ttl: %+v) %+v ==> %v", ttl, req, ok)
	}

}
