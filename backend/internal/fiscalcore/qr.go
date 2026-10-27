package fiscalcore

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
)

type QRConfiguration struct {
	BaseURL string
	Version int
	CSCID   string
	CSC     string
}

func queryURL(base, value string) (string, error) {
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Hostname() == "" || !strings.HasSuffix(strings.ToLower(u.Hostname()), ".gov.br") || u.RawQuery != "" || u.Fragment != "" {
		return "", ErrInput
	}
	q := url.Values{}
	q.Set("p", value)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// QRCode derives parameters exclusively from a verified signed NFC-e. Version
// and URL come from trusted UF/ambiente configuration, not inferred nationally.
// The CSC is used only in the v2 hash; it is never included in the returned URL.
func (c *Credential) QRCode(config QRConfiguration, signed []byte, now time.Time) (string, error) {
	if err := c.VerifyXML("nfe", signed, now); err != nil {
		return "", err
	}
	d, _ := parseXML(signed)
	info, _ := payload(d.Root(), "nfe")
	key := strings.TrimPrefix(info.SelectAttrValue("Id", ""), "NFe")
	get := func(p string) string {
		e := info.FindElement(p)
		if e == nil {
			return ""
		}
		return e.Text()
	}
	if !ValidAccessKey(key) || key[20:22] != "65" {
		return "", ErrInput
	}
	env, emission := get("ide/tpAmb"), get("ide/tpEmis")
	if env != "1" && env != "2" {
		return "", ErrInput
	}
	issued, err := time.Parse(time.RFC3339, get("ide/dhEmi"))
	if err != nil {
		return "", ErrXML
	}
	if emission != "1" && emission != "9" {
		return "", ErrUnsupported
	}
	value := key + "|" + strconv.Itoa(config.Version) + "|" + env
	if config.Version == 3 {
		if emission == "9" {
			destType, destID := "", ""
			if id := get("dest/CNPJ"); id != "" {
				destType, destID = "1", id
			} else if id := get("dest/CPF"); id != "" {
				destType, destID = "2", id
			} else if get("dest/idEstrangeiro") != "" {
				destType = "3"
			}
			value += fmt.Sprintf("|%02d|%s|%s|%s", issued.Day(), get("total/ICMSTot/vNF"), destType, destID)
			signature, e := c.signBytes([]byte(value), now)
			if e != nil {
				return "", e
			}
			value += "|" + base64.StdEncoding.EncodeToString(signature)
		}
	} else if config.Version == 2 {
		if len(config.CSCID) < 1 || len(config.CSCID) > 6 || !digitsRE.MatchString(config.CSCID) || len(config.CSC) < 16 || len(config.CSC) > 128 {
			return "", ErrCredential
		}
		id, e := strconv.Atoi(config.CSCID)
		if e != nil || id < 1 {
			return "", ErrCredential
		}
		if emission == "9" {
			digest := d.Root().FindElement("Signature/SignedInfo/Reference/DigestValue")
			if digest == nil {
				return "", ErrXML
			}
			value += fmt.Sprintf("|%02d|%s|%s", issued.Day(), get("total/ICMSTot/vNF"), hex.EncodeToString([]byte(digest.Text())))
		}
		value += "|" + strconv.Itoa(id)
		hash := sha1.Sum([]byte(value + config.CSC))
		value += "|" + strings.ToUpper(hex.EncodeToString(hash[:]))
	} else {
		return "", ErrUnsupported
	}
	return queryURL(config.BaseURL, value)
}

func QRPNG(value string) ([]byte, error) {
	if len(value) == 0 || len(value) > 1024 {
		return nil, ErrInput
	}
	code, err := qr.Encode(value, qr.M, qr.Unicode)
	if err != nil {
		return nil, ErrInput
	}
	// Four-module quiet zone, four pixels/module, no interpolated scaling.
	side := code.Bounds().Dx() * 4
	scaled, err := barcode.Scale(code, side, side)
	if err != nil {
		return nil, ErrInput
	}
	out := image.NewRGBA(image.Rect(0, 0, side+32, side+32))
	draw.Draw(out, out.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(out, image.Rect(16, 16, side+16, side+16), scaled, image.Point{}, draw.Src)
	var b bytes.Buffer
	if err = png.Encode(&b, out); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
