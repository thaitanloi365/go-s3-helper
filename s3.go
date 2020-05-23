package s3

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// GenerateSignature generate signature form
func (wrapper *Wrapper) GenerateSignature() Signature {
	var cfg = wrapper.Config

	t := time.Now().Add(time.Minute * time.Duration(cfg.PolicyOptions.ExpiryMinutes))
	formattedShortTime := t.UTC().Format(shortTimeFormat)
	date := t.UTC().Format(timeFormat)
	cred := fmt.Sprintf("%s/%s/%s/s3/aws4_request", cfg.Credentials.AccessKey, formattedShortTime, cfg.Credentials.Region)
	b64Policy := fmt.Sprintf(policyDocument,
		t.UTC().Format(expirationFormat),
		cfg.Credentials.Bucket,
		"",
		cfg.PolicyOptions.MaxFileSize,
		cred,
		date,
	)

	// Generate policy
	policy := base64.StdEncoding.EncodeToString([]byte(b64Policy))

	// Generate signature
	h1 := makeHmac([]byte("AWS4"+cfg.Credentials.SecretKey), []byte(date[:8]))
	h2 := makeHmac(h1, []byte(cfg.Credentials.Region))
	h3 := makeHmac(h2, []byte("s3"))
	h4 := makeHmac(h3, []byte("aws4_request"))
	signature := hex.EncodeToString(makeHmac(h4, []byte(policy)))

	// Base url
	url := fmt.Sprintf("https://%s.s3.amazonaws.com/", cfg.Credentials.Bucket)
	return Signature{
		URL:        url,
		ACL:        acl,
		Algorithm:  algorithm,
		Credential: cred,
		Date:       date,
		Policy:     policy,
		Signature:  signature,
	}
}

// Helper to make the HMAC-SHA256.
func makeHmac(key []byte, data []byte) []byte {
	hash := hmac.New(sha256.New, key)
	hash.Write(data)
	return hash.Sum(nil)
}
