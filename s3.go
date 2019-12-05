package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

const policyDocument = `
{ "expiration": "%s",
  "conditions": [
    {"bucket": "%s"},
    ["starts-with", "$key", "%s"],
    {"acl": "public-read"},
    ["content-length-range", 1, %d],
    {"x-amz-credential": "%s"},
    {"x-amz-algorithm": "AWS4-HMAC-SHA256"},
    {"x-amz-date": "%s" }
  ]
}
`
const (
	expirationFormat = "2006-01-02T15:04:05.000Z"
	timeFormat       = "20060102T150405Z"
	shortTimeFormat  = "20060102"
	acl              = "public-read"
	algorithm        = "AWS4-HMAC-SHA256"
)

// Represents AWS credentials and config.
type Credentials struct {
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
}

// Represents policy options.
type PolicyOptions struct {
	ExpiryMinutes int
	MaxFileSize   int
}

// Config config
type Config struct {
	Credentials   Credentials
	PolicyOptions PolicyOptions
}

// Signature response
type Signature struct {
	URL        string `json:"URL"`
	Policy     string `json:"Policy"`
	Credential string `json:"X-Amz-Credential"`
	Algorithm  string `json:"X-Amz-Algorithm"`
	Signature  string `json:"X-Amz-Signature"`
	Date       string `json:"X-Amz-Date"`
	ACL        string `json:"ACL"`
}

// GenerateSignature generate signature form
func (config *Config) GenerateSignature() Signature {
	t := time.Now().Add(time.Minute * time.Duration(config.PolicyOptions.ExpiryMinutes))
	formattedShortTime := t.UTC().Format(shortTimeFormat)
	date := t.UTC().Format(timeFormat)
	cred := fmt.Sprintf("%s/%s/%s/s3/aws4_request", config.Credentials.AccessKeyID, formattedShortTime, config.Credentials.Region)
	b64Policy := fmt.Sprintf(policyDocument,
		t.UTC().Format(expirationFormat),
		config.Credentials.Bucket,
		"",
		config.PolicyOptions.MaxFileSize,
		cred,
		date,
	)

	// Generate policy
	policy := base64.StdEncoding.EncodeToString([]byte(b64Policy))

	// Generate signature
	h1 := makeHmac([]byte("AWS4"+config.Credentials.SecretAccessKey), []byte(date[:8]))
	h2 := makeHmac(h1, []byte(config.Credentials.Region))
	h3 := makeHmac(h2, []byte("s3"))
	h4 := makeHmac(h3, []byte("aws4_request"))
	signature := hex.EncodeToString(makeHmac(h4, []byte(policy)))

	// Base url
	url := fmt.Sprintf("https://%s.s3.amazonaws.com/", config.Credentials.Bucket)
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
